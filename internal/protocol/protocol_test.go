package protocol

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
)

func TestWriteOpenRequestAppendsNewline(t *testing.T) {
	var buf bytes.Buffer
	err := WriteOpenRequest(&buf, OpenRequest{Token: "secret", RemoteHost: "10.0.0.15", RemotePort: 1433})
	if err != nil {
		t.Fatalf("WriteOpenRequest() error = %v", err)
	}

	if got := buf.String(); got[len(got)-1] != '\n' {
		t.Fatalf("WriteOpenRequest() did not append newline: %q", got)
	}
}

func TestReadOpenRequestKeepsBufferedTraffic(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	go func() {
		_, _ = client.Write([]byte(`{"remote_host":"db.internal","remote_port":1433}` + "\nSELECT 1"))
		_ = client.Close()
	}()

	req, reader, err := ReadOpenRequest(server)
	if err != nil {
		t.Fatalf("ReadOpenRequest() error = %v", err)
	}
	if req.RemoteHost != "db.internal" || req.RemotePort != 1433 {
		t.Fatalf("ReadOpenRequest() = %#v", req)
	}

	buffered, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll(buffered) error = %v", err)
	}
	if string(buffered) != "SELECT 1" {
		t.Fatalf("buffered traffic = %q", buffered)
	}
}

func TestOpenRequestValidate(t *testing.T) {
	for _, req := range []OpenRequest{
		{RemotePort: 1433},
		{RemoteHost: "db.internal"},
		{RemoteHost: "db.internal", RemotePort: 70000},
	} {
		if err := req.Validate(); err == nil {
			t.Fatalf("Validate() succeeded for invalid request %#v", req)
		}
	}
}

func TestReadOpenRequestRejectsOversizedMessage(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	go func() {
		_, _ = client.Write(bytes.Repeat([]byte{'a'}, maxHeaderBytes+1))
	}()

	if _, _, err := ReadOpenRequest(server); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("ReadOpenRequest() error = %v, want message-too-large error", err)
	}
}

func TestMaximumSizeOpenRequestRoundTrip(t *testing.T) {
	req := OpenRequest{Token: strings.Repeat("a", maxHeaderBytes), RemoteHost: "x", RemotePort: 1}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	req.Token = req.Token[:len(req.Token)-(len(data)-(maxHeaderBytes-1))]

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()
	errCh := make(chan error, 1)
	go func() { errCh <- WriteOpenRequest(client, req) }()

	got, _, err := ReadOpenRequest(server)
	if err != nil {
		t.Fatalf("ReadOpenRequest() error = %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("WriteOpenRequest() error = %v", err)
	}
	if got != req {
		t.Fatal("request changed during round trip")
	}
}

func TestReadOpenRequestRejectsUnknownFields(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()
	go func() {
		_, _ = client.Write([]byte(`{"remote_host":"db.internal","remote_port":1433,"extra":true}` + "\n"))
	}()

	if _, _, err := ReadOpenRequest(server); err == nil {
		t.Fatal("ReadOpenRequest() accepted an unknown field")
	}
}

func TestOpenResponseRoundTrip(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()
	errCh := make(chan error, 1)
	go func() { errCh <- WriteOpenResponse(client, OpenResponse{OK: true}) }()

	response, _, err := ReadOpenResponse(server)
	if err != nil {
		t.Fatalf("ReadOpenResponse() error = %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("WriteOpenResponse() error = %v", err)
	}
	if !response.OK {
		t.Fatalf("ReadOpenResponse() = %#v", response)
	}
}

func TestReadOpenResponseKeepsBufferedTraffic(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()
	go func() {
		_, _ = client.Write([]byte("{\"ok\":true}\nREADY"))
		_ = client.Close()
	}()

	response, reader, err := ReadOpenResponse(server)
	if err != nil || !response.OK {
		t.Fatalf("ReadOpenResponse() = %#v, %v", response, err)
	}
	buffered, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(buffered) != "READY" {
		t.Fatalf("buffered traffic = %q", buffered)
	}
}
