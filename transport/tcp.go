package transport

import "net"

type tcpTransport struct {
	conn net.Conn
}

// TCP creates a transport from a TCP connection.
func TCP(conn net.Conn) Transport {
	return &tcpTransport{conn: conn}
}

func (t *tcpTransport) Read(p []byte) (int, error)  { return t.conn.Read(p) }
func (t *tcpTransport) Write(p []byte) (int, error) { return t.conn.Write(p) }
func (t *tcpTransport) Close() error                { return t.conn.Close() }

// ListenTCP starts a TCP listener and returns the first connection as a transport.
// This is the typical mode for LSP servers accepting a single client connection.
func ListenTCP(addr string) (Transport, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	defer ln.Close()
	conn, err := ln.Accept()
	if err != nil {
		return nil, err
	}
	return TCP(conn), nil
}
