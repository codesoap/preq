preq is an HTTP requester for the [httpipe
format](https://github.com/codesoap/httpipe). It takes httpipe input via
standard input, makes the given requests and prints httpipe to standard
output.

preq is designed to be used with HTTP 1.1 requests only and does not
support the CONNECT method.

# Examples
```console
$ echo '{"host":"x.com","req":"GET / HTTP/1.1\\r\\nHost: x.com\\r\\n\\r\\n"}' | preq
{"host":"x.com","port":443,"req":"GET / HTTP/1.1\r\nHost: x.com\r\n\r\n","resp":"HTTP/1.1 302 Moved Temporarily\r\n ... "}

$ # Using pfuzz to generate requests and drip to limit the request rate:
$ pfuzz -w /path/to/wordlist -u 'https://foo.com/FUZZ' | drip 500ms | preq -p 20
...
```

# Installation
You can download precompiled binaries from the [releases
page](https://github.com/codesoap/preq/releases) or install it with
`go install github.com/codesoap/preq@latest`.

# Usage
```console
$ preq -h
Usage of preq:
  -p int
        Number of parallel requests. (default 1)
  -t duration
        Timeout for requests. (default 5s)

preq expects input via standard input in the httpipe format. At least
the "host" and "req" fields must be present. If the "tls" field is
missing, TLS (HTTPS) will be used. If the "port" field is missing, port
80 will be used if TLS is not used and port 443 otherwise.

preq will make requests in the order they arrived via standard input.
However, if the value of the -p flag is greater than 1, the order of the
output lines may not match the input.

Example:
echo '{"host":"x.com","req":"GET / HTTP/1.1\\r\\nHost: x.com\\r\\n\\r\\n"}' | preq
```
