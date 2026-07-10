package protocol

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
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

type OpenResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
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
	return writeMessage(w, req)
}

func ReadOpenRequest(conn net.Conn) (OpenRequest, *bufio.Reader, error) {
	reader := bufio.NewReaderSize(conn, maxHeaderBytes)
	var req OpenRequest
	if err := readMessage(conn, reader, &req); err != nil {
		return OpenRequest{}, nil, err
	}
	if err := req.Validate(); err != nil {
		return OpenRequest{}, nil, err
	}
	return req, reader, nil
}

func WriteOpenResponse(w io.Writer, response OpenResponse) error {
	if response.OK && response.Error != "" {
		return fmt.Errorf("successful response cannot contain an error")
	}
	if !response.OK && strings.TrimSpace(response.Error) == "" {
		return fmt.Errorf("failed response must contain an error")
	}
	return writeMessage(w, response)
}

func ReadOpenResponse(conn net.Conn) (OpenResponse, *bufio.Reader, error) {
	reader := bufio.NewReaderSize(conn, maxHeaderBytes)
	var response OpenResponse
	if err := readMessage(conn, reader, &response); err != nil {
		return OpenResponse{}, nil, err
	}
	if response.OK && response.Error != "" {
		return OpenResponse{}, nil, fmt.Errorf("successful response contains an error")
	}
	if !response.OK && strings.TrimSpace(response.Error) == "" {
		return OpenResponse{}, nil, fmt.Errorf("failed response does not contain an error")
	}
	return response, reader, nil
}

func writeMessage(w io.Writer, message any) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if len(data) > maxHeaderBytes-1 {
		return fmt.Errorf("protocol message is too large")
	}

	data = append(data, '\n')
	written, err := w.Write(data)
	if err == nil && written != len(data) {
		return io.ErrShortWrite
	}
	return err
}

func readMessage(conn net.Conn, reader *bufio.Reader, target any) error {
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()

	line, err := reader.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) {
		return fmt.Errorf("protocol message is too large")
	}
	if err != nil {
		return err
	}
	if len(line) > maxHeaderBytes {
		return fmt.Errorf("protocol message is too large")
	}

	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("protocol message contains multiple values")
		}
		return err
	}
	return nil
}
