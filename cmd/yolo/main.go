package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	"github.com/kido5217/yolo/internal/auth"
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
		return authCmd(args[1:])
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
	port := fs.Int("port", 4096, "port to listen on (0 = ephemeral)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	s := server.NewServer(server.Deps{WorkDir: mustGetwd()})
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

func authUsage() int {
	fmt.Fprintln(os.Stderr, "Usage:\n  yolo auth list\n  yolo auth add <provider> [key]\n  yolo auth remove <provider>")
	return 2
}

func authCmd(args []string) int {
	if len(args) == 0 {
		return authUsage()
	}
	sub, rest := args[0], args[1:]

	loadStore := func() (auth.Store, error) { return auth.Load() }

	switch sub {
	case "list":
		s, err := loadStore()
		if err != nil {
			fmt.Fprintln(os.Stderr, "auth list:", err)
			return 1
		}
		if len(s) == 0 {
			fmt.Println("no credentials")
			return 0
		}
		ids := make([]string, 0, len(s))
		for id := range s {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			fmt.Printf("%s  %s  (set)\n", id, s[id].Type)
		}
		return 0
	case "add":
		if len(rest) < 1 {
			return authUsage()
		}
		provider := rest[0]
		key := ""
		if len(rest) >= 2 {
			key = rest[1]
		} else {
			// no new dep: plain stdin prompt, echo NOT disabled (documented limitation)
			fmt.Fprint(os.Stderr, "API key: ")
			line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			key = strings.TrimSpace(line)
		}
		s, err := loadStore()
		if err != nil {
			fmt.Fprintln(os.Stderr, "auth add:", err)
			return 1
		}
		s.Set(provider, key)
		if err := auth.Save(s); err != nil {
			fmt.Fprintln(os.Stderr, "auth add:", err)
			return 1
		}
		return 0
	case "remove":
		if len(rest) < 1 {
			return authUsage()
		}
		s, err := loadStore()
		if err != nil {
			fmt.Fprintln(os.Stderr, "auth remove:", err)
			return 1
		}
		s.Delete(rest[0])
		if err := auth.Save(s); err != nil {
			fmt.Fprintln(os.Stderr, "auth remove:", err)
			return 1
		}
		return 0
	default:
		return authUsage()
	}
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}
