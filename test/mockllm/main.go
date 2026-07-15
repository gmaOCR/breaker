// Command mockllm is a fake upstream that speaks Anthropic SSE with fixed, known
// token usage — it lets the e2e test drive the breaker offline, with no API key.
// Each response reports input=1000, output=1000 tokens (≈ $0.018 on Sonnet-5).
package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
)

func main() {
	addr := "127.0.0.1:8991"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mockllm:", err)
		os.Exit(1)
	}
	fmt.Println(ln.Addr().String()) // first stdout line: the actual address
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/messages", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		io.WriteString(w, "event: message_start\n"+
			`data: {"type":"message_start","message":{"model":"claude-sonnet-5","usage":{"input_tokens":1000,"output_tokens":1}}}`+"\n\n")
		if fl != nil {
			fl.Flush()
		}
		io.WriteString(w, "event: message_delta\n"+
			`data: {"type":"message_delta","usage":{"output_tokens":1000}}`+"\n\n")
		io.WriteString(w, "event: message_stop\n"+`data: {"type":"message_stop"}`+"\n\n")
		if fl != nil {
			fl.Flush()
		}
	})
	_ = http.Serve(ln, mux)
}
