package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/codesoap/preq/extractor"
)

// TODO: do not touch lines with already filled err or resp?!

var usageDetails = `
preq expects input via standard input in the httpipe format. At least
the "host" and "req" fields must be present. If the "tls" field is
missing, TLS (HTTPS) will be used. If the "port" field is missing, port
80 will be used if TLS is not used and port 443 otherwise.

preq will make requests in the order they arrived via standard input.
However, if the value of the -p flag is greater than 1, the order of the
output lines may not match the input.

Example:
echo '{"host":"x.com","req":"GET / HTTP/1.1\\r\\nHost: x.com\\r\\n\\r\\n"}' | preq
`

var timeout time.Duration
var pFlag int

type httpline struct {
	Host string `json:"host"`
	Port int    `json:"port,omitempty"`
	TLS  *bool  `json:"tls,omitempty"`
	Req  string `json:"req"`

	Reqat *jtime `json:"reqat,omitempty"`
	Ping  int64  `json:"ping,omitempty"`
	Resp  string `json:"resp,omitempty"`
	Err   string `json:"err,omitempty"`
	Errno int    `json:"errno,omitempty"`
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprint(flag.CommandLine.Output(), usageDetails)
	}

	flag.DurationVar(&timeout, "t", 5*time.Second, "Timeout for requests.")
	flag.IntVar(&pFlag, "p", 1, "Number of parallel requests.")
	flag.Parse()
}

func main() {
	requests := make(chan httpline)
	go readLines(requests)

	results := make(chan httpline)
	var wg sync.WaitGroup
	for i := 0; i < pFlag; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			doRequests(requests, results)
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	printResults(results)
}

func readLines(lines chan httpline) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		rawLine := scanner.Bytes()
		var line httpline
		err := json.Unmarshal(rawLine, &line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Could not parse line '%s': %v\n", rawLine, err)
			os.Exit(1)
		}
		lines <- line
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Error: Could not read standard input:", err)
		os.Exit(1)
	}
	close(lines)
}

func doRequests(requests, results chan httpline) {
	for request := range requests {
		results <- doRequest(request)
	}
}

func doRequest(request httpline) httpline {
	setDefaultTLSAndPortIfNecessary(&request)
	deadline := time.Now().Add(timeout)
	conn, err := getConn(request, deadline)
	if err != nil {
		request.Errno, request.Err = toErrno(err), err.Error()
		return request
	}
	defer conn.Close()
	if err = conn.SetDeadline(deadline); err != nil {
		request.Errno, request.Err = 99, err.Error()
		return request
	}
	_, err = fmt.Fprint(conn, request.Req)
	if err != nil {
		// FIXME: errno 30 may not be ideal.
		request.Errno, request.Err = 30, err.Error()
		return request
	}
	now := jtime(time.Now())
	request.Reqat = &now
	timedConn := &timedReader{r: conn}
	resp, err := extractor.ExtractResponse(timedConn, isHEAD(request.Req))
	request.Resp = resp
	if !timedConn.readAt.IsZero() {
		request.Ping = timedConn.readAt.Sub(time.Time(*request.Reqat)).Milliseconds()
	}
	if err != nil {
		request.Errno, request.Err = 99, err.Error()
		return request
	}
	return request
}

func toErrno(err error) int {
	if errors.Is(err, context.DeadlineExceeded) {
		return 31
	}
	switch err2 := err.(type) {
	case *net.OpError:
		switch err3 := err2.Err.(type) {
		case *net.DNSError:
			if err3.IsNotFound {
				return 10
			} else if err3.IsTimeout {
				return 11
			}
		case *os.SyscallError:
			if err3.Err == syscall.ECONNREFUSED {
				return 30
			}
		case net.Error:
			if err3.Timeout() {
				return 31
			}
		}
	case *tls.CertificateVerificationError:
		return 20 // FIXME: Cannot distinguish different TLS errors.
	}
	return 99 // Undefined errno for unknown error.
}

func setDefaultTLSAndPortIfNecessary(request *httpline) {
	if request.TLS == nil {
		t := true
		request.TLS = &t
	}
	if request.Port == 0 {
		if !*request.TLS {
			request.Port = 80
		} else {
			request.Port = 443
		}
	}
}

func getConn(request httpline, deadline time.Time) (net.Conn, error) {
	dialer := net.Dialer{Deadline: deadline}
	addr := fmt.Sprintf("%s:%d", request.Host, request.Port)
	if request.TLS != nil && !*request.TLS {
		return dialer.Dial("tcp", addr)
	}
	return tls.DialWithDialer(&dialer, "tcp", addr, nil)
}

func isHEAD(req string) bool {
	return len(req) >= len("HEAD") && strings.ToLower(req[:len("HEAD")]) == "head"
}

func printResults(results chan httpline) {
	for result := range results {
		out, err := json.Marshal(result)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: Could not generate result:", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	}
}
