// package extractor provides a function for extracting an HTTP response
// from an io.Reader. This is necessary, because with keep-alive
// connections, the response may end before the reader is closed.
package extractor

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// TODO: Improve conformance with RFC.
// TODO: Improve performance/reduce memory allocations.

const maxBufSize = 1024

// ExtractResponse extracts the response from a reader.
//
// It mostly adheres to RFC 7230, section 3.3.3., but is more lax at
// times. For example, \n is also accepted instead of \r\n in some
// places.
func ExtractResponse(in io.Reader, headRequest bool) (string, error) {
	var out strings.Builder
	reader := bufio.NewReader(in)
	contentLength, chunked, noBody, err := readHead(reader, &out)
	if err != nil || headRequest || noBody {
		return out.String(), err
	}
	if chunked {
		err = readChunkedBody(reader, &out)
	} else if contentLength != nil {
		err = copyN(reader, &out, *contentLength)
	} else {
		_, err = io.Copy(&out, reader)
	}
	return out.String(), err
}

func readHead(in *bufio.Reader, out io.Writer) (*int64, bool, bool, error) {
	var contentLength *int64
	var chunked bool
	line, err := readAndCopyLine(in, out)
	if err != nil {
		return nil, false, false, fmt.Errorf("could not read status line: %w", err)
	}
	noBody := hasNoBodyStatusCode(line)
	for {
		line, err := readAndCopyLine(in, out)
		if err != nil {
			return nil, false, false, fmt.Errorf("could not read line: %w", err)
		}
		if line == "" {
			break
		}
		lowerLine := strings.ToLower(line)
		if strings.HasPrefix(lowerLine, "content-length:") {
			if contentLength != nil {
				return nil, false, false, fmt.Errorf("multiple Content-Length headers found")
			}
			n := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			i, err := strconv.ParseInt(n, 10, 64)
			if err != nil {
				return nil, false, false, fmt.Errorf("invalid Content-Length in '%s': %w", line, err)
			}
			contentLength = &i
		}
		if strings.HasPrefix(lowerLine, "transfer-encoding:") {
			fields := strings.Split(strings.SplitN(line, ":", 2)[1], ",")
			chunked = strings.TrimSpace(fields[len(fields)-1]) == "chunked"
		}
	}
	return contentLength, chunked, noBody, nil
}

func hasNoBodyStatusCode(statusLine string) bool {
	fields := strings.Fields(statusLine)
	if len(fields) < 2 {
		return false
	}
	statusCode, err := strconv.Atoi(fields[1])
	if err != nil {
		return false
	}
	return statusCode >= 100 && statusCode < 200 || statusCode == 204 || statusCode == 304
}

func readChunkedBody(in *bufio.Reader, out io.Writer) error {
	for {
		chunk, err := readAndCopyLine(in, out)
		if err != nil {
			return err
		}
		chunkSize, err := strconv.ParseInt(strings.Split(chunk, ";")[0], 16, 64)
		if err != nil {
			return fmt.Errorf("invalid chunk '%s'", chunk)
		}
		if chunkSize == 0 {
			break
		}
		// +2 ist for \r\n that must come at the end of each chunk.
		if err = copyN(in, out, chunkSize+2); err != nil {
			return fmt.Errorf("could not read full chunk body: %w", err)
		}
	}
	for {
		line, err := readAndCopyLine(in, out)
		if err != nil {
			return err
		}
		if line == "" {
			return nil
		}
	}
}

func readAndCopyLine(in *bufio.Reader, out io.Writer) (string, error) {
	rawLine, err := in.ReadBytes('\n')
	if err != nil {
		return "", fmt.Errorf("could not read line: %w", err)
	}
	if _, err := out.Write(rawLine); err != nil {
		return "", fmt.Errorf("could not write line: %w", err)
	}
	return strings.TrimRight(string(rawLine), "\r\n"), nil
}

func copyN(in *bufio.Reader, out io.Writer, n int64) error {
	if n == 0 {
		return nil
	}
	for n > maxBufSize {
		buf := make([]byte, maxBufSize)
		read, err := in.Read(buf)
		if err != nil {
			return err
		}
		_, err = out.Write(buf[:read])
		if err != nil {
			return err
		}
		n -= int64(read)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(in, buf); err != nil {
		return err
	}
	_, err := out.Write(buf)
	return err
}
