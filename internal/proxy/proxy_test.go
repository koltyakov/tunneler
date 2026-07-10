package proxy

import (
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

var errWrite = errors.New("write failed")

type funcConn struct {
	read      func([]byte) (int, error)
	write     func([]byte) (int, error)
	closed    chan struct{}
	closeOnce sync.Once
}

func (c *funcConn) Read(p []byte) (int, error)       { return c.read(p) }
func (c *funcConn) Write(p []byte) (int, error)      { return c.write(p) }
func (c *funcConn) LocalAddr() net.Addr              { return testAddr("local") }
func (c *funcConn) RemoteAddr() net.Addr             { return testAddr("remote") }
func (c *funcConn) SetDeadline(time.Time) error      { return nil }
func (c *funcConn) SetReadDeadline(time.Time) error  { return nil }
func (c *funcConn) SetWriteDeadline(time.Time) error { return nil }
func (c *funcConn) Close() error                     { c.closeOnce.Do(func() { close(c.closed) }); return nil }

type testAddr string

func (a testAddr) Network() string { return string(a) }
func (a testAddr) String() string  { return string(a) }

func TestPipeUnblocksOtherDirectionAfterCopyError(t *testing.T) {
	aClosed := make(chan struct{})
	a := &funcConn{
		closed: aClosed,
		read: func([]byte) (int, error) {
			<-aClosed
			return 0, io.EOF
		},
		write: func([]byte) (int, error) { return 0, errWrite },
	}
	bClosed := make(chan struct{})
	readOnce := sync.Once{}
	b := &funcConn{
		closed: bClosed,
		read: func(p []byte) (int, error) {
			read := false
			readOnce.Do(func() {
				p[0] = 'x'
				read = true
			})
			if read {
				return 1, nil
			}
			<-bClosed
			return 0, io.EOF
		},
		write: func(p []byte) (int, error) { return len(p), nil },
	}

	done := make(chan error, 1)
	go func() { done <- Pipe(a, b) }()
	select {
	case err := <-done:
		if !errors.Is(err, errWrite) {
			t.Fatalf("Pipe() error = %v, want %v", err, errWrite)
		}
	case <-time.After(time.Second):
		t.Fatal("Pipe() did not unblock after a copy error")
	}
}
