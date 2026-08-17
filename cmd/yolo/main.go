package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/kido5217/yolo/internal/server"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "yolo: TUI not wired yet (milestone M6); use `yolo serve`")
		return 0
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(os.Stderr, `yolo — Go port of opencode (v1.18.18 wire contract)

Usage:
  yolo [<sessionID>]        start the TUI (or resume a session)
  yolo serve [--port N]     run the core server only
  yolo auth <subcommand>    manage credentials (list | add <provider> [key] | remove <provider>)
  yolo help                 this help
`)
		return 0
	case "serve":
		return serve(args[1:])
	case "auth":
		fmt.Fprintln(os.Stderr, "yolo auth: not wired yet (Task 4)")
		return 0
	case "version":
		fmt.Println("yolo 0.0.0-dev")
		return 0
	default:
		fmt.Fprintf(os.Stderr, "yolo: resume not wired yet (Task 27); unknown argument %q\n", args[0])
		return 2
	}
}

func serve(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 0, "port to listen on (0 = ephemeral)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	s := server.New(mustGetwd())
	addr, err := s.Start(fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		fmt.Fprintln(os.Stderr, "listen:", err)
		return 1
	}
	srv := addr.(*net.TCPAddr)
	fmt.Printf("yolo serving on http://%s (dir %s)\n", srv.String(), s.WorkDir)
	stop := make(chan os.Signal, 1)
	// import os/signal here is not allowed pre-M8; block on channel closed by Close
	<-stop
	s.Close()
	return 0
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}
