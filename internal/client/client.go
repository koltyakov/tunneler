package client

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/koltyakov/tunneler/internal/config"
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
	defer cancel()

	var wg sync.WaitGroup
	errCh := make(chan error, len(c.cfg.Tunnels))
	for _, tunnel := range c.cfg.Tunnels {
		wg.Add(1)
		go func(t config.Tunnel) {
			defer wg.Done()
			if err := c.runTunnel(runCtx, t); err != nil && runCtx.Err() == nil {
				errCh <- err
			}
		}(tunnel)
	}

	go func() {
		wg.Wait()
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		return nil
	case err, ok := <-errCh:
		if !ok {
			return nil
		}
		cancel()
		return err
	}
}

func (c *Client) runTunnel(ctx context.Context, tunnel config.Tunnel) error {
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

	for {
		local, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			c.log.Warn("accept failed", "local", tunnel.LocalAddress(), "error", err)
			continue
		}
		go c.handle(tunnel, local)
	}
}

func (c *Client) handle(tunnel config.Tunnel, local net.Conn) {
	defer func() { _ = local.Close() }()

	serverConn, err := net.DialTimeout("tcp", c.cfg.ServerAddress, 15*time.Second)
	if err != nil {
		c.log.Warn("server dial failed", "server", c.cfg.ServerAddress, "tunnel", tunnel.Label(), "error", err)
		return
	}
	defer func() { _ = serverConn.Close() }()

	req := protocol.OpenRequest{Token: c.cfg.Token, RemoteHost: tunnel.TargetHost, RemotePort: tunnel.TargetPort}
	if err := protocol.WriteOpenRequest(serverConn, req); err != nil {
		c.log.Warn("open request failed", "server", c.cfg.ServerAddress, "tunnel", tunnel.Label(), "error", err)
		return
	}

	c.log.Info("connection opened", "tunnel", tunnel.Label(), "local", local.RemoteAddr().String())
	proxy.Pipe(local, serverConn)
	c.log.Info("connection closed", "tunnel", tunnel.Label(), "local", local.RemoteAddr().String())
}

func Run(ctx context.Context, cfg config.Client, logger *slog.Logger) error {
	return New(cfg, logger).Run(ctx)
}
