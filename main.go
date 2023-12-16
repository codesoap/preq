package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/codesoap/preq/extractor"
)

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

var tlsConfig = tls.Config{}
var timeout time.Duration
var pFlag int

type httpline struct {
	Host string `json:"host"`
	Port int    `json:"port,omitempty"`
	TLS  *bool  `json:"tls,omitempty"`
	Req  string `json:"req"`

	Resp  string     `json:"resp,omitempty"`
	Ping  int        `json:"ping,omitempty"` // TODO
	Time  *time.Time `json:"time,omitempty"` // TODO
	Err   string     `json:"err,omitempty"`
	Errno int        `json:"errno,omitempty"`
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
	conn, err := getConn(request)
	if err != nil {
		request.Errno, request.Err = 1, err.Error()
		return request
	}
	defer conn.Close()
	if err = conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		request.Errno, request.Err = 100, err.Error()
		return request
	}
	_, err = fmt.Fprint(conn, request.Req)
	if err != nil {
		request.Errno, request.Err = 10, err.Error()
		return request
	}
	resp, err := extractor.ExtractResponse(conn, isHEAD(request.Req))
	request.Resp = resp
	if err != nil {
		request.Errno, request.Err = 20, err.Error()
		return request
	}
	return request
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

func getConn(request httpline) (net.Conn, error) {
	addr := fmt.Sprintf("%s:%d", request.Host, request.Port)
	if request.TLS != nil && !*request.TLS {
		return net.Dial("tcp", addr)
	}
	return tls.Dial("tcp", addr, &tlsConfig)
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
