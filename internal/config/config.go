package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/koltyakov/tunneler/internal/protocol"
)

type Server struct {
	ListenAddress  string   `json:"listenAddress"`
	Token          string   `json:"token"`
	MaxConnections int      `json:"maxConnections,omitempty"`
	AllowedTargets []string `json:"allowedTargets,omitempty"`
}

type Client struct {
	ServerAddress  string   `json:"serverAddress"`
	Token          string   `json:"token"`
	MaxConnections int      `json:"maxConnections,omitempty"`
	Tunnels        []Tunnel `json:"tunnels"`
}

const defaultMaxConnections = 128

type Tunnel struct {
	Name       string `json:"name,omitempty"`
	LocalPort  int    `json:"localPort,omitempty"`
	TargetHost string `json:"targetHost"`
	TargetPort int    `json:"targetPort"`
}

func LoadServer(path string) (Server, error) {
	var cfg Server
	if err := load(path, &cfg); err != nil {
		return Server{}, err
	}
	if cfg.ListenAddress == "" {
		cfg.ListenAddress = ":7000"
	}
	if cfg.MaxConnections == 0 {
		cfg.MaxConnections = defaultMaxConnections
	}
	if err := cfg.Validate(); err != nil {
		return Server{}, err
	}
	return cfg, nil
}

func LoadClient(path string) (Client, error) {
	var cfg Client
	if err := load(path, &cfg); err != nil {
		return Client{}, err
	}
	if cfg.MaxConnections == 0 {
		cfg.MaxConnections = defaultMaxConnections
	}
	return cfg, nil
}

func (c Server) Validate() error {
	if strings.TrimSpace(c.ListenAddress) == "" {
		return fmt.Errorf("listenAddress is required")
	}
	if strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("token is required")
	}
	if c.MaxConnections < 0 {
		return fmt.Errorf("maxConnections cannot be negative")
	}
	for i, address := range c.AllowedTargets {
		host, portText, err := net.SplitHostPort(address)
		if err != nil || strings.TrimSpace(host) == "" {
			return fmt.Errorf("allowedTargets[%d] must be a host:port address", i)
		}
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("allowedTargets[%d] port must be between 1 and 65535", i)
		}
	}
	return nil
}

func (c Client) Validate() error {
	if strings.TrimSpace(c.ServerAddress) == "" {
		return fmt.Errorf("serverAddress is required")
	}
	if strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("token is required")
	}
	if c.MaxConnections < 0 {
		return fmt.Errorf("maxConnections cannot be negative")
	}
	if len(c.Tunnels) == 0 {
		return fmt.Errorf("at least one tunnel is required")
	}

	seen := map[string]struct{}{}
	for i, tunnel := range c.Tunnels {
		localPort := tunnel.EffectiveLocalPort()
		if localPort < 1 || localPort > 65535 {
			return fmt.Errorf("tunnels[%d].localPort must be between 1 and 65535", i)
		}
		localAddress := tunnel.LocalAddress()
		if _, ok := seen[localAddress]; ok {
			return fmt.Errorf("duplicate localPort %d", localPort)
		}
		seen[localAddress] = struct{}{}

		req := protocol.OpenRequest{RemoteHost: tunnel.TargetHost, RemotePort: tunnel.TargetPort}
		if err := req.Validate(); err != nil {
			return fmt.Errorf("tunnels[%d]: %w", i, err)
		}
	}
	return nil
}

func (c Server) EffectiveMaxConnections() int {
	if c.MaxConnections == 0 {
		return defaultMaxConnections
	}
	return c.MaxConnections
}

func (c Server) AllowsTarget(address string) bool {
	if len(c.AllowedTargets) == 0 {
		return true
	}
	for _, allowed := range c.AllowedTargets {
		if address == allowed {
			return true
		}
	}
	return false
}

func (c Client) EffectiveMaxConnections() int {
	if c.MaxConnections == 0 {
		return defaultMaxConnections
	}
	return c.MaxConnections
}

func (t Tunnel) Label() string {
	if t.Name != "" {
		return t.Name
	}
	return t.LocalAddress() + " -> " + net.JoinHostPort(t.TargetHost, strconv.Itoa(t.TargetPort))
}

func (t Tunnel) EffectiveLocalPort() int {
	if t.LocalPort != 0 {
		return t.LocalPort
	}
	return t.TargetPort
}

func (t Tunnel) LocalAddress() string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(t.EffectiveLocalPort()))
}

func load(path string, target any) error {
	// #nosec G304 -- the caller explicitly selects the configuration file.
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
