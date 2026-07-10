package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClientValidate(t *testing.T) {
	cfg := Client{
		ServerAddress: "vpn.example.com:7000",
		Token:         "secret",
		Tunnels: []Tunnel{
			{TargetHost: "10.0.0.15", TargetPort: 1433},
			{LocalPort: 15432, TargetHost: "10.0.0.20", TargetPort: 5432},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestClientValidateRejectsDuplicateLocalPort(t *testing.T) {
	cfg := Client{
		ServerAddress: "vpn.example.com:7000",
		Token:         "secret",
		Tunnels: []Tunnel{
			{TargetHost: "10.0.0.15", TargetPort: 1433},
			{LocalPort: 1433, TargetHost: "10.0.0.20", TargetPort: 5432},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() succeeded with duplicate localPort")
	}
}

func TestTunnelLabel(t *testing.T) {
	named := Tunnel{Name: "mssql", TargetHost: "10.0.0.15", TargetPort: 1433}
	if got := named.Label(); got != "mssql" {
		t.Fatalf("Label() = %q", got)
	}

	unnamed := Tunnel{TargetHost: "10.0.0.15", TargetPort: 1433}
	if got := unnamed.Label(); got != "127.0.0.1:1433 -> 10.0.0.15:1433" {
		t.Fatalf("Label() = %q", got)
	}
}

func TestTunnelLocalAddressUsesOptionalLocalPort(t *testing.T) {
	defaulted := Tunnel{TargetHost: "10.0.0.15", TargetPort: 1433}
	if got := defaulted.LocalAddress(); got != "127.0.0.1:1433" {
		t.Fatalf("LocalAddress() = %q", got)
	}

	overridden := Tunnel{LocalPort: 11433, TargetHost: "10.0.0.15", TargetPort: 1433}
	if got := overridden.LocalAddress(); got != "127.0.0.1:11433" {
		t.Fatalf("LocalAddress() = %q", got)
	}
}

func TestLoadServerRejectsUnknownField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.json")
	if err := os.WriteFile(path, []byte(`{"listenAddress":":7000","tokne":"secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadServer(path); err == nil {
		t.Fatal("LoadServer() accepted an unknown field")
	}
}

func TestLoadServerRequiresToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.json")
	if err := os.WriteFile(path, []byte(`{"listenAddress":":7000"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadServer(path); err == nil {
		t.Fatal("LoadServer() accepted a missing token")
	}
}

func TestLoadServerAppliesConnectionLimitDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.json")
	if err := os.WriteFile(path, []byte(`{"token":"secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadServer(path)
	if err != nil {
		t.Fatalf("LoadServer() error = %v", err)
	}
	if cfg.MaxConnections != defaultMaxConnections {
		t.Fatalf("MaxConnections = %d, want %d", cfg.MaxConnections, defaultMaxConnections)
	}
}

func TestServerAllowsConfiguredTargets(t *testing.T) {
	cfg := Server{AllowedTargets: []string{"10.0.0.15:1433"}}
	if !cfg.AllowsTarget("10.0.0.15:1433") {
		t.Fatal("AllowsTarget() rejected configured target")
	}
	if cfg.AllowsTarget("10.0.0.20:1433") {
		t.Fatal("AllowsTarget() accepted unconfigured target")
	}
}
