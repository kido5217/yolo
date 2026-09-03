// Command mock is the S8.1 parity mock runner: it binds 127.0.0.1:0,
// prints MOCK_PORT=<port> (the handshake line the S8.2 capture reads),
// and serves the canned book for -turn (text|tool|todo). The capture
// starts it per surface and kills it when the pty closes.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/kido5217/yolo/internal/llm/mockllm"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "listen address (0 = ephemeral port)")
	turn := flag.String("turn", "text", "canned turn: text | tool | todo")
	canned := flag.String("canned", "", "canned book JSON path (default: the built-in book)")
	flag.Parse()

	book := mockllm.DefaultBook()
	if *canned != "" {
		raw, err := os.ReadFile(*canned)
		if err != nil {
			log.Fatalf("canned: %v", err)
		}
		if book, err = mockllm.LoadBook(raw); err != nil {
			log.Fatalf("canned: %v", err)
		}
	}
	var c mockllm.Canned
	switch *turn {
	case "text":
		c = book.Text
	case "tool":
		c = book.Tool
	case "todo":
		c = book.Todo
	default:
		log.Fatalf("unknown turn %q (text|tool|todo)", *turn)
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: mockllm.Handler(c)}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()
	fmt.Printf("MOCK_PORT=%d\n", ln.Addr().(*net.TCPAddr).Port)
	select {} // the capture kills the process
}
