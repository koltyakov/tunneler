package protocol

import (
	"bytes"
	"io"
	"net"
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
