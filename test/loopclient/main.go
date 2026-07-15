// Command loopclient hammers the proxied LLM endpoint forever, ignoring HTTP
// errors on purpose: the ONLY thing that stops it is the breaker's SIGKILL.
// That is exactly what the e2e test asserts.
package main

import (
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	base := os.Getenv("ANTHROPIC_BASE_URL")
	if base == "" {
		os.Exit(3)
	}
	for {
		resp, err := http.Post(base+"/v1/messages", "application/json",
			strings.NewReader(`{"model":"claude-sonnet-5","stream":true}`))
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}
}
