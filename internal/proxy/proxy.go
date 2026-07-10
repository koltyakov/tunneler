package proxy

import (
	"errors"
	"io"
	"net"
)

func Pipe(a net.Conn, b net.Conn) error {
	errs := make(chan error, 2)
	go func() { errs <- copyAndClose(a, b) }()
	go func() { errs <- copyAndClose(b, a) }()

	first := <-errs
	if first != nil {
		_ = a.Close()
		_ = b.Close()
	}
	second := <-errs
	return errors.Join(first, second)
}

func copyAndClose(dst net.Conn, src net.Conn) error {
	_, err := io.Copy(dst, src)
	if err != nil {
		return err
	}
	_ = closeWrite(dst)
	_ = closeRead(src)
	return nil
}

func closeWrite(conn net.Conn) error {
	if tcp, ok := conn.(*net.TCPConn); ok {
		return tcp.CloseWrite()
	}
	return conn.Close()
}

func closeRead(conn net.Conn) error {
	if tcp, ok := conn.(*net.TCPConn); ok {
		return tcp.CloseRead()
	}
	return conn.Close()
}
