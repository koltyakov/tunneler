package server

import (
	"context"
	"crypto/hmac"
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

type Server struct {
	cfg config.Server
	log *slog.Logger
}

func New(cfg config.Server, logger *slog.Logger) *Server {
	return &Server{cfg: cfg, log: logger}
}

func (s *Server) Run(ctx context.Context) error {
	if err := s.cfg.Validate(); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	listener, err := net.Listen("tcp", s.cfg.ListenAddress)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()

	s.log.Info("server listening", "addr", listener.Addr().String())
	if len(s.cfg.AllowedTargets) == 0 {
		s.log.Warn("target allowlist is disabled; authenticated clients may reach any target")
	}
	go func() {
		<-runCtx.Done()
		_ = listener.Close()
	}()

	connections := connset.New()
	slots := make(chan struct{}, s.cfg.EffectiveMaxConnections())
	var handlers sync.WaitGroup
	backoff := time.Duration(0)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if runCtx.Err() != nil || errors.Is(err, net.ErrClosed) {
				break
			}
			backoff = nextBackoff(backoff)
			s.log.Warn("accept failed", "error", err, "retry_in", backoff)
			if !waitForRetry(runCtx, backoff) {
				break
			}
			continue
		}
		backoff = 0
		select {
		case slots <- struct{}{}:
		default:
			s.log.Warn("connection limit reached", "remote", conn.RemoteAddr().String())
			_ = conn.Close()
			continue
		}
		if !connections.Add(conn) {
			<-slots
			break
		}
		handlers.Add(1)
		go func() {
			defer handlers.Done()
			defer func() { <-slots }()
			defer connections.Remove(conn)
			s.handle(runCtx, conn, connections)
		}()
	}

	cancel()
	connections.Close()
	handlers.Wait()
	return nil
}

func (s *Server) handle(ctx context.Context, client net.Conn, connections *connset.Set) {
	defer func() { _ = client.Close() }()

	req, reader, err := protocol.ReadOpenRequest(client)
	if err != nil {
		s.log.Warn("invalid open request", "remote", client.RemoteAddr().String(), "error", err)
		_ = writeOpenResponse(client, protocol.OpenResponse{Error: "invalid open request"})
		return
	}
	if !hmac.Equal([]byte(req.Token), []byte(s.cfg.Token)) {
		s.log.Warn("authentication failed", "remote", client.RemoteAddr().String(), "target", req.RemoteAddress())
		_ = writeOpenResponse(client, protocol.OpenResponse{Error: "authentication failed"})
		return
	}
	if !s.cfg.AllowsTarget(req.RemoteAddress()) {
		s.log.Warn("target rejected", "remote", client.RemoteAddr().String(), "target", req.RemoteAddress())
		_ = writeOpenResponse(client, protocol.OpenResponse{Error: "target is not allowed"})
		return
	}

	dialer := net.Dialer{Timeout: 15 * time.Second}
	target, err := dialer.DialContext(ctx, "tcp", req.RemoteAddress())
	if err != nil {
		s.log.Warn("target dial failed", "target", req.RemoteAddress(), "error", err)
		_ = writeOpenResponse(client, protocol.OpenResponse{Error: "target connection failed"})
		return
	}
	if !connections.Add(target) {
		return
	}
	defer connections.Remove(target)
	defer func() { _ = target.Close() }()
	if err := writeOpenResponse(client, protocol.OpenResponse{OK: true}); err != nil {
		s.log.Warn("open response failed", "client", client.RemoteAddr().String(), "error", err)
		return
	}

	s.log.Info("tunnel opened", "client", client.RemoteAddr().String(), "target", req.RemoteAddress())
	if buffered := reader.Buffered(); buffered > 0 {
		if _, err := io.CopyN(target, reader, int64(buffered)); err != nil {
			s.log.Warn("buffer flush failed", "target", req.RemoteAddress(), "error", err)
			return
		}
	}
	if err := proxy.Pipe(client, target); err != nil && ctx.Err() == nil {
		s.log.Debug("tunnel copy failed", "client", client.RemoteAddr().String(), "target", req.RemoteAddress(), "error", err)
	}
	s.log.Info("tunnel closed", "client", client.RemoteAddr().String(), "target", req.RemoteAddress())
}

func Run(ctx context.Context, cfg config.Server, logger *slog.Logger) error {
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

func writeOpenResponse(conn net.Conn, response protocol.OpenResponse) error {
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	defer func() { _ = conn.SetWriteDeadline(time.Time{}) }()
	return protocol.WriteOpenResponse(conn, response)
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
