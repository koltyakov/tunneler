package server

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/koltyakov/tunneler/internal/config"
	"github.com/koltyakov/tunneler/internal/connset"
	"github.com/koltyakov/tunneler/internal/protocol"
)

func TestHandleRejectsInvalidToken(t *testing.T) {
	client, serverConn := net.Pipe()
	connections := connset.New()
	connections.Add(serverConn)
	done := make(chan struct{})
	go func() {
		New(config.Server{Token: "secret"}, discardLogger()).handle(context.Background(), serverConn, connections)
		close(done)
	}()
	defer func() { _ = client.Close() }()

	if err := protocol.WriteOpenRequest(client, protocol.OpenRequest{Token: "wrong", RemoteHost: "127.0.0.1", RemotePort: 1}); err != nil {
		t.Fatal(err)
	}
	response, _, err := protocol.ReadOpenResponse(client)
	if err != nil {
		t.Fatalf("ReadOpenResponse() error = %v", err)
	}
	if response.OK || response.Error != "authentication failed" {
		t.Fatalf("response = %#v", response)
	}
	waitDone(t, done)
}

func TestHandleReportsTargetDialFailure(t *testing.T) {
	target := listenTCP(t)
	port := target.Addr().(*net.TCPAddr).Port
	_ = target.Close()

	client, serverConn := net.Pipe()
	connections := connset.New()
	connections.Add(serverConn)
	done := make(chan struct{})
	go func() {
		New(config.Server{Token: "secret"}, discardLogger()).handle(context.Background(), serverConn, connections)
		close(done)
	}()
	defer func() { _ = client.Close() }()

	if err := protocol.WriteOpenRequest(client, protocol.OpenRequest{Token: "secret", RemoteHost: "127.0.0.1", RemotePort: port}); err != nil {
		t.Fatal(err)
	}
	response, _, err := protocol.ReadOpenResponse(client)
	if err != nil {
		t.Fatalf("ReadOpenResponse() error = %v", err)
	}
	if response.OK || response.Error != "target connection failed" {
		t.Fatalf("response = %#v", response)
	}
	waitDone(t, done)
}

func TestHandleForwardsTrafficAfterSuccessResponse(t *testing.T) {
	target := listenTCP(t)
	defer func() { _ = target.Close() }()
	targetDone := make(chan struct{})
	go func() {
		defer close(targetDone)
		conn, err := target.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		buf := make([]byte, 4)
		if _, err := io.ReadFull(conn, buf); err == nil {
			_, _ = conn.Write(buf)
		}
	}()

	client, serverConn := net.Pipe()
	connections := connset.New()
	connections.Add(serverConn)
	done := make(chan struct{})
	go func() {
		New(config.Server{Token: "secret"}, discardLogger()).handle(context.Background(), serverConn, connections)
		close(done)
	}()

	port := target.Addr().(*net.TCPAddr).Port
	if err := protocol.WriteOpenRequest(client, protocol.OpenRequest{Token: "secret", RemoteHost: "127.0.0.1", RemotePort: port}); err != nil {
		t.Fatal(err)
	}
	response, _, err := protocol.ReadOpenResponse(client)
	if err != nil || !response.OK {
		t.Fatalf("ReadOpenResponse() = %#v, %v", response, err)
	}
	if _, err := client.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 4)
	if _, err := io.ReadFull(client, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "ping" {
		t.Fatalf("forwarded response = %q", got)
	}
	_ = client.Close()
	waitDone(t, done)
	waitDone(t, targetDone)
}

func listenTCP(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return listener
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func waitDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for goroutine")
	}
}
