package config

import "testing"

func TestClientValidate(t *testing.T) {
	cfg := Client{
		ServerAddress: "vpn.example.com:7000",
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
