package connset

import (
	"net"
	"sync"
)

type Set struct {
	mu      sync.Mutex
	closing bool
	conns   map[net.Conn]struct{}
}

func New() *Set {
	return &Set{conns: make(map[net.Conn]struct{})}
}

func (s *Set) Add(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		_ = conn.Close()
		return false
	}
	s.conns[conn] = struct{}{}
	return true
}

func (s *Set) Remove(conn net.Conn) {
	s.mu.Lock()
	delete(s.conns, conn)
	s.mu.Unlock()
}

func (s *Set) Close() {
	s.mu.Lock()
	s.closing = true
	connections := make([]net.Conn, 0, len(s.conns))
	for conn := range s.conns {
		connections = append(connections, conn)
		delete(s.conns, conn)
	}
	s.mu.Unlock()
	for _, conn := range connections {
		_ = conn.Close()
	}
}
