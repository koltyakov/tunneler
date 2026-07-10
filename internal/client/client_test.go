package client

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/koltyakov/tunneler/internal/config"
	"github.com/koltyakov/tunneler/internal/protocol"
)

func TestRunCancellationClosesActiveConnection(t *testing.T) {
	server, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	serverClosed := make(chan struct{})
	go func() {
		defer close(serverClosed)
		conn, acceptErr := server.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		if _, _, readErr := protocol.ReadOpenRequest(conn); readErr != nil {
			return
		}
		if writeErr := protocol.WriteOpenResponse(conn, protocol.OpenResponse{OK: true}); writeErr != nil {
			return
		}
		_, _ = io.Copy(io.Discard, conn)
	}()

	localPort := reservePort(t)
	cfg := config.Client{
		ServerAddress: server.Addr().String(),
		Token:         "secret",
		Tunnels: []config.Tunnel{{
			LocalPort:  localPort,
			TargetHost: "db.internal",
			TargetPort: 1433,
		}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		runDone <- Run(ctx, cfg, logger)
	}()

	local := dialEventually(t, net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)))
	defer func() { _ = local.Close() }()
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not stop after cancellation")
	}
	select {
	case <-serverClosed:
	case <-time.After(2 * time.Second):
		t.Fatal("server connection remained open after client cancellation")
	}
}

func reservePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}

func dialEventually(t *testing.T, address string) net.Conn {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", address, 50*time.Millisecond)
		if err == nil {
			return conn
		}
		if time.Now().After(deadline) {
			t.Fatalf("dial %s: %v", address, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
