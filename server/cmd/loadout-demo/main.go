// loadout-demo seeds an explicitly selected local server, or serves mock MCP tools.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/demo"
)

func main() {
	origin := flag.String("origin", "", "Local Web/API origin to seed (one-shot, empty demo data required)")
	upstream := flag.String("upstream", "", "Local demo MCP URL to include in seed data")
	listen := flag.String("listen", "", "Serve demo MCP tools on a loopback address, e.g. 127.0.0.1:8790")
	more := flag.Bool("more", false, "Expand an existing small demo dataset once")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var err error
	if *listen != "" && *origin == "" {
		err = serve(ctx, *listen)
	} else if *origin != "" && *listen == "" {
		seedCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if *more {
			err = demo.Expand(seedCtx, *origin, os.Getenv("LOADOUT_ADMIN_USERNAME"), os.Getenv("LOADOUT_ADMIN_PASSWORD"))
		} else {
			err = demo.Seed(seedCtx, *origin, *upstream, os.Getenv("LOADOUT_ADMIN_USERNAME"), os.Getenv("LOADOUT_ADMIN_PASSWORD"))
		}
		if err == nil {
			fmt.Println("Demo data ready. Accounts: demo_owner, demo_member, demo_empty, demo_disabled (disabled).")
		}
	} else {
		err = errors.New("choose either -origin to seed or -listen to serve mock MCP")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve(ctx context.Context, addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		return errors.New("mock MCP must listen on a loopback IP")
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.New("mock MCP listen failed")
	}
	server := &http.Server{Handler: demo.Upstream(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 40 * time.Second, IdleTimeout: 60 * time.Second}
	defer func() { _ = server.Close() }()
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	fmt.Printf("Demo MCP ready: http://%s\n", listener.Addr())
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return errors.New("mock MCP server failed")
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
	return nil
}
