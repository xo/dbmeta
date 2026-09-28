package container

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// bashRequest is a bash command that sends one HTTP request to a port inside
// the container and succeeds when the answer's status is one of want. A setup
// step that makes an object often wants two: the status for made, and the
// status for already there.
//
// Several images have bash and neither curl nor wget, such as Qdrant's and
// Chroma's. bash opens a TCP connection itself through /dev/tcp, so the
// request is written there and the status line is read back. HTTP/1.0 asks
// the server to close the connection, so that the read ends.
//
// The body, when there is one, is sent as JSON. Each header is written as
// given, and the headers are sorted so that the command is the same on every
// run.
func bashRequest(port int, method, path, body string, headers map[string]string, want ...int) []string {
	var h strings.Builder
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&h, `%s: %s\r\n`, k, headers[k])
	}
	if body != "" {
		h.WriteString(`Content-Type: application/json\r\n`)
	}
	codes := make([]string, len(want))
	for i, w := range want {
		codes[i] = strconv.Itoa(w)
	}
	script := fmt.Sprintf(`b=%s
exec 3<>/dev/tcp/127.0.0.1/%d || exit 1
printf '%s %s HTTP/1.0\r\nHost: 127.0.0.1\r\n%sContent-Length: %%d\r\n\r\n%%s' "${#b}" "$b" >&3
read -r _ status _ <&3
case "$status" in %s) ;; *) exit 1 ;; esac`, shellQuote(body), port, printfLiteral(method), printfLiteral(path),
		printfLiteral(h.String()), strings.Join(codes, "|"))
	return []string{"bash", "-c", script}
}

// printfLiteral escapes the percent signs in text that goes into the format
// of printf, so that a path such as /q?query=ASK%7B%7D reaches the server as
// it is written.
func printfLiteral(s string) string {
	return strings.ReplaceAll(s, "%", "%%")
}

// shellQuote quotes a string for a POSIX shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// bashEither runs the second bash command only when the first fails, such as
// a request that makes an object after one that asks whether it is there.
// Each runs in a subshell of its own, because each is several lines and ||
// joins only the lines next to it.
func bashEither(first, second []string) []string {
	return []string{"bash", "-c", "(" + first[2] + ") || (" + second[2] + ")"}
}

// bashAll runs bash commands one after another and stops at the first that
// fails. Each runs in a subshell of its own, for the reason bashEither gives.
func bashAll(commands ...[]string) []string {
	parts := make([]string, len(commands))
	for i, c := range commands {
		parts[i] = "(" + c[2] + ")"
	}
	return []string{"bash", "-c", strings.Join(parts, " &&\n")}
}

// adminBasic is the Authorization header of HTTP basic authentication for
// admin with [Password].
var adminBasic = "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:"+Password))
