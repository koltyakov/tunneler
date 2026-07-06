package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/koltyakov/tunneler/internal/protocol"
)

type Server struct {
	ListenAddress string `json:"listenAddress"`
	Token         string `json:"token,omitempty"`
}

type Client struct {
	ServerAddress string   `json:"serverAddress"`
	Token         string   `json:"token,omitempty"`
	Tunnels       []Tunnel `json:"tunnels"`
}

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
	return cfg, nil
}

func LoadClient(path string) (Client, error) {
	var cfg Client
	if err := load(path, &cfg); err != nil {
		return Client{}, err
	}
	return cfg, nil
}

func (c Client) Validate() error {
	if strings.TrimSpace(c.ServerAddress) == "" {
		return fmt.Errorf("serverAddress is required")
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
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
