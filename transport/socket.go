package transport

import (
	"net"
	"os"
)

// ListenSocket starts a Unix domain socket listener and returns the first
// connection as a transport. Used by Neovim's vim.lsp.rpc.connect() and
// other editors supporting local IPC.
func ListenSocket(path string) (Transport, error) {
	os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	defer ln.Close()
	conn, err := ln.Accept()
	if err != nil {
		return nil, err
	}
	return &socketTransport{conn: conn, path: path}, nil
}

type socketTransport struct {
	conn net.Conn
	path string
}

func (s *socketTransport) Read(p []byte) (int, error)  { return s.conn.Read(p) }
func (s *socketTransport) Write(p []byte) (int, error) { return s.conn.Write(p) }
func (s *socketTransport) Close() error {
	err := s.conn.Close()
	os.Remove(s.path)
	return err
}
