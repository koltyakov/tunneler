package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const maxHeaderBytes = 64 * 1024

type OpenRequest struct {
	Token      string `json:"token,omitempty"`
	RemoteHost string `json:"remote_host"`
	RemotePort int    `json:"remote_port"`
}

func (r OpenRequest) RemoteAddress() string {
	return net.JoinHostPort(r.RemoteHost, strconv.Itoa(r.RemotePort))
}

func (r OpenRequest) Validate() error {
	if strings.TrimSpace(r.RemoteHost) == "" {
		return fmt.Errorf("remote_host is required")
	}
	if r.RemotePort < 1 || r.RemotePort > 65535 {
		return fmt.Errorf("remote_port must be between 1 and 65535")
	}
	return nil
}

func WriteOpenRequest(w io.Writer, req OpenRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if len(data) > maxHeaderBytes-1 {
		return fmt.Errorf("open request is too large")
	}

	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

func ReadOpenRequest(conn net.Conn) (OpenRequest, *bufio.Reader, error) {
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReaderSize(conn, maxHeaderBytes)
	line, err := reader.ReadString('\n')
	_ = conn.SetReadDeadline(time.Time{})
	if err != nil {
		return OpenRequest{}, nil, err
	}
	if len(line) >= maxHeaderBytes {
		return OpenRequest{}, nil, fmt.Errorf("open request is too large")
	}

	var req OpenRequest
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		return OpenRequest{}, nil, err
	}
	if err := req.Validate(); err != nil {
		return OpenRequest{}, nil, err
	}
	return req, reader, nil
}
