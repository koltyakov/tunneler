package proxy

import (
	"io"
	"net"
	"sync"
)

func Pipe(a net.Conn, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go copyAndClose(&wg, a, b)
	go copyAndClose(&wg, b, a)

	wg.Wait()
}

func copyAndClose(wg *sync.WaitGroup, dst net.Conn, src net.Conn) {
	defer wg.Done()
	_, _ = io.Copy(dst, src)
	_ = closeWrite(dst)
	_ = closeRead(src)
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
