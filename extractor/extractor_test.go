package extractor_test

import (
	"strings"
	"testing"

	"github.com/codesoap/preq/extractor"
)

type testCase struct {
	isHEAD      bool
	in          string
	expectedOut string
	expectedErr bool
}

var tests = []testCase{
	{
		false,
		"HTTP/1.1 404 Not Found\r\nContent-Type: text/plain; charset=utf-8\r\nX-Content-Type-Options: nosniff\r\nDate: Sun, 17 Dec 2023 12:05:16 GMT\r\nContent-Length: 19\r\n\r\n404 page not found\n foo",
		"HTTP/1.1 404 Not Found\r\nContent-Type: text/plain; charset=utf-8\r\nX-Content-Type-Options: nosniff\r\nDate: Sun, 17 Dec 2023 12:05:16 GMT\r\nContent-Length: 19\r\n\r\n404 page not found\n",
		false,
	},
	{
		false,
		"HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 10\r\n\r\nAll good.\n",
		"HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 10\r\n\r\nAll good.\n",
		false,
	},
	{
		true,
		"HTTP/1.1 200 OK\r\nContent-Length: 10\r\n\r\nAll good.\n",
		"HTTP/1.1 200 OK\r\nContent-Length: 10\r\n\r\n",
		false,
	},
	{
		false,
		"HTTP/1.1 200 OK\r\n\r\n404 page not found\n",
		"HTTP/1.1 200 OK\r\n\r\n404 page not found\n",
		false,
	},
	{
		false,
		"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\na\r\nAll good.\n\r\n0\r\n\r\n",
		"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\na\r\nAll good.\n\r\n0\r\n\r\n",
		false,
	},
	{
		false,
		"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\na\r\nAll good.\n\r\n4\r\nfoo\n\r\n0\r\n\r\n",
		"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\na\r\nAll good.\n\r\n4\r\nfoo\n\r\n0\r\n\r\n",
		false,
	},
	{
		false,
		"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\na\r\nAll good.\n\r\n0\r\nTrailer: foo\r\n\r\n",
		"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\na\r\nAll good.\n\r\n0\r\nTrailer: foo\r\n\r\n",
		false,
	},
	{
		false,
		"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\na\r\nAll good.\n\r\n0\r\n\r\nfoo",
		"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\na\r\nAll good.\n\r\n0\r\n\r\n",
		false,
	},
	{
		false,
		"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n1\r\nAll good.\n\r\n0\r\n\r\n",
		"",
		true,
	},
	{
		false,
		"HTTP/1.1 200 OK\r\nContent-Length: 1\r\nTransfer-Encoding: chunked\r\n\r\na\r\nAll good.\n\r\n0\r\n\r\n",
		"HTTP/1.1 200 OK\r\nContent-Length: 1\r\nTransfer-Encoding: chunked\r\n\r\na\r\nAll good.\n\r\n0\r\n\r\n",
		false,
	},
	// TODO: More tests with errors due to invalid sizes.
	// TODO: Test when Content-Length and Transfer-Encoding are present.
	// TODO: Ensure correct differentiation between \n and \r\n.
	// TODO: Ensure correct handling of responses to CONNECT requests?
}

func TestExtraction(t *testing.T) {
	for i, tt := range tests {
		resp, err := extractor.ExtractResponse(strings.NewReader(tt.in), tt.isHEAD)
		if err != nil && !tt.expectedErr {
			t.Errorf("%d. Got unexpected error: %v", i, err)
		} else if err == nil && resp != tt.expectedOut {
			t.Errorf("Got unexpected extract.\nGot   : %s\nWanted: %s", resp, tt.expectedOut)
		}
	}
}
