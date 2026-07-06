package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/koltyakov/tunneler/internal/client"
	"github.com/koltyakov/tunneler/internal/config"
	"github.com/koltyakov/tunneler/internal/server"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	args := os.Args[1:]
	if len(args) > 0 && args[0] == "server" {
		runServer(ctx, logger, args[1:])
		return
	}
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		usage()
		return
	}

	runClient(ctx, logger, args)
}

func runClient(ctx context.Context, logger *slog.Logger, args []string) {
	fs := flag.NewFlagSet("tunneler", flag.ExitOnError)
	configPath := fs.String("config", "client.json", "path to client JSON config")
	serverAddr := fs.String("server", "", "override tunnel server address")
	token := fs.String("token", "", "override auth token")
	fs.Usage = clientUsage
	_ = fs.Parse(args)

	cfg, err := config.LoadClient(*configPath)
	if err != nil {
		logger.Error("load config failed", "error", err)
		os.Exit(1)
	}
	if *serverAddr != "" {
		cfg.ServerAddress = *serverAddr
	}
	if *token != "" {
		cfg.Token = *token
	}

	if err := client.Run(ctx, cfg, logger); err != nil {
		logger.Error("client stopped", "error", err)
		os.Exit(1)
	}
}

func runServer(ctx context.Context, logger *slog.Logger, args []string) {
	fs := flag.NewFlagSet("tunneler server", flag.ExitOnError)
	configPath := fs.String("config", "server.json", "path to server JSON config")
	listenAddr := fs.String("listen", "", "override listen address, for example :7000")
	token := fs.String("token", "", "override auth token")
	fs.Usage = serverUsage
	_ = fs.Parse(args)

	cfg, err := config.LoadServer(*configPath)
	if err != nil {
		logger.Error("load config failed", "error", err)
		os.Exit(1)
	}
	if *listenAddr != "" {
		cfg.ListenAddress = *listenAddr
	}
	if *token != "" {
		cfg.Token = *token
	}

	if err := server.Run(ctx, cfg, logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  tunneler [client flags]")
	fmt.Fprintln(os.Stderr, "  tunneler server [server flags]")
	fmt.Fprintln(os.Stderr)
	clientUsage()
	fmt.Fprintln(os.Stderr)
	serverUsage()
}

func clientUsage() {
	fmt.Fprintln(os.Stderr, "Client flags:")
	fmt.Fprintln(os.Stderr, "  -config string")
	fmt.Fprintln(os.Stderr, "        path to client JSON config (default \"client.json\")")
	fmt.Fprintln(os.Stderr, "  -server string")
	fmt.Fprintln(os.Stderr, "        override tunnel server address")
	fmt.Fprintln(os.Stderr, "  -token string")
	fmt.Fprintln(os.Stderr, "        override auth token")
}

func serverUsage() {
	fmt.Fprintln(os.Stderr, "Server flags:")
	fmt.Fprintln(os.Stderr, "  -config string")
	fmt.Fprintln(os.Stderr, "        path to server JSON config (default \"server.json\")")
	fmt.Fprintln(os.Stderr, "  -listen string")
	fmt.Fprintln(os.Stderr, "        override listen address, for example :7000")
	fmt.Fprintln(os.Stderr, "  -token string")
	fmt.Fprintln(os.Stderr, "        override auth token")
}
