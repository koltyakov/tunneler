package client

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/koltyakov/tunneler/internal/config"
	"github.com/koltyakov/tunneler/internal/connset"
	"github.com/koltyakov/tunneler/internal/protocol"
	"github.com/koltyakov/tunneler/internal/proxy"
)

type Client struct {
	cfg config.Client
	log *slog.Logger
}

func New(cfg config.Client, logger *slog.Logger) *Client {
	return &Client{cfg: cfg, log: logger}
}

func (c *Client) Run(ctx context.Context) error {
	if err := c.cfg.Validate(); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)

	connections := connset.New()
	slots := make(chan struct{}, c.cfg.EffectiveMaxConnections())
	var tunnels sync.WaitGroup
	var handlers sync.WaitGroup
	errCh := make(chan error, len(c.cfg.Tunnels))
	for _, tunnel := range c.cfg.Tunnels {
		tunnels.Add(1)
		go func(t config.Tunnel) {
			defer tunnels.Done()
			if err := c.runTunnel(runCtx, t, connections, slots, &handlers); err != nil && runCtx.Err() == nil {
				errCh <- err
			}
		}(tunnel)
	}

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errCh:
	}
	cancel()
	tunnels.Wait()
	connections.Close()
	handlers.Wait()
	return runErr
}

func (c *Client) runTunnel(ctx context.Context, tunnel config.Tunnel, connections *connset.Set, slots chan struct{}, handlers *sync.WaitGroup) error {
	listener, err := net.Listen("tcp", tunnel.LocalAddress())
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()

	c.log.Info("tunnel listening", "name", tunnel.Label(), "local", listener.Addr().String(), "target", tunnel.TargetHost, "target_port", tunnel.TargetPort)
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	backoff := time.Duration(0)
	for {
		local, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			backoff = nextBackoff(backoff)
			c.log.Warn("accept failed", "local", tunnel.LocalAddress(), "error", err, "retry_in", backoff)
			if !waitForRetry(ctx, backoff) {
				return nil
			}
			continue
		}
		backoff = 0
		select {
		case slots <- struct{}{}:
		default:
			c.log.Warn("connection limit reached", "tunnel", tunnel.Label())
			_ = local.Close()
			continue
		}
		if !connections.Add(local) {
			<-slots
			return nil
		}
		handlers.Add(1)
		go func() {
			defer handlers.Done()
			defer func() { <-slots }()
			defer connections.Remove(local)
			c.handle(ctx, tunnel, local, connections)
		}()
	}
}

func (c *Client) handle(ctx context.Context, tunnel config.Tunnel, local net.Conn, connections *connset.Set) {
	defer func() { _ = local.Close() }()

	dialer := net.Dialer{Timeout: 15 * time.Second}
	serverConn, err := dialer.DialContext(ctx, "tcp", c.cfg.ServerAddress)
	if err != nil {
		c.log.Warn("server dial failed", "server", c.cfg.ServerAddress, "tunnel", tunnel.Label(), "error", err)
		return
	}
	if !connections.Add(serverConn) {
		return
	}
	defer connections.Remove(serverConn)
	defer func() { _ = serverConn.Close() }()

	req := protocol.OpenRequest{Token: c.cfg.Token, RemoteHost: tunnel.TargetHost, RemotePort: tunnel.TargetPort}
	_ = serverConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := protocol.WriteOpenRequest(serverConn, req); err != nil {
		c.log.Warn("open request failed", "server", c.cfg.ServerAddress, "tunnel", tunnel.Label(), "error", err)
		return
	}
	_ = serverConn.SetWriteDeadline(time.Time{})
	response, reader, err := protocol.ReadOpenResponse(serverConn)
	if err != nil {
		c.log.Warn("open response failed", "server", c.cfg.ServerAddress, "tunnel", tunnel.Label(), "error", err)
		return
	}
	if !response.OK {
		c.log.Warn("tunnel rejected", "server", c.cfg.ServerAddress, "tunnel", tunnel.Label(), "error", response.Error)
		return
	}
	if buffered := reader.Buffered(); buffered > 0 {
		if _, err := io.CopyN(local, reader, int64(buffered)); err != nil {
			c.log.Warn("response buffer flush failed", "server", c.cfg.ServerAddress, "tunnel", tunnel.Label(), "error", err)
			return
		}
	}

	c.log.Info("connection opened", "tunnel", tunnel.Label(), "local", local.RemoteAddr().String())
	if err := proxy.Pipe(local, serverConn); err != nil && ctx.Err() == nil {
		c.log.Debug("connection copy failed", "tunnel", tunnel.Label(), "local", local.RemoteAddr().String(), "error", err)
	}
	c.log.Info("connection closed", "tunnel", tunnel.Label(), "local", local.RemoteAddr().String())
}

func Run(ctx context.Context, cfg config.Client, logger *slog.Logger) error {
	return New(cfg, logger).Run(ctx)
}

func nextBackoff(current time.Duration) time.Duration {
	if current == 0 {
		return 5 * time.Millisecond
	}
	current *= 2
	if current > time.Second {
		return time.Second
	}
	return current
}

func waitForRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
