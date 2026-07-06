package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/koltyakov/tunneler/internal/config"
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
	listener, err := net.Listen("tcp", s.cfg.ListenAddress)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()

	s.log.Info("server listening", "addr", listener.Addr().String())
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			s.log.Warn("accept failed", "error", err)
			continue
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(client net.Conn) {
	defer func() { _ = client.Close() }()

	req, reader, err := protocol.ReadOpenRequest(client)
	if err != nil {
		s.log.Warn("invalid open request", "remote", client.RemoteAddr().String(), "error", err)
		return
	}
	if s.cfg.Token != "" && req.Token != s.cfg.Token {
		s.log.Warn("authentication failed", "remote", client.RemoteAddr().String(), "target", req.RemoteAddress())
		return
	}

	target, err := net.DialTimeout("tcp", req.RemoteAddress(), 15*time.Second)
	if err != nil {
		s.log.Warn("target dial failed", "target", req.RemoteAddress(), "error", err)
		return
	}
	defer func() { _ = target.Close() }()

	s.log.Info("tunnel opened", "client", client.RemoteAddr().String(), "target", req.RemoteAddress())
	if buffered := reader.Buffered(); buffered > 0 {
		if _, err := io.CopyN(target, reader, int64(buffered)); err != nil {
			s.log.Warn("buffer flush failed", "target", req.RemoteAddress(), "error", err)
			return
		}
	}
	proxy.Pipe(client, target)
	s.log.Info("tunnel closed", "client", client.RemoteAddr().String(), "target", req.RemoteAddress())
}

func Run(ctx context.Context, cfg config.Server, logger *slog.Logger) error {
	if cfg.ListenAddress == "" {
		return fmt.Errorf("listenAddress is required")
	}
	return New(cfg, logger).Run(ctx)
}
