package transport

import "net"

// ListenPipe starts a named pipe (or Unix socket on non-Windows) listener.
// On Windows, this uses named pipes; on Unix, it falls back to Unix domain sockets.
// The VS Code extension host and Neovim on Windows use this transport.
func ListenPipe(name string) (Transport, error) {
	return ListenSocket(name)
}

// DialPipe connects to an existing named pipe / Unix domain socket.
func DialPipe(name string) (Transport, error) {
	conn, err := net.Dial("unix", name)
	if err != nil {
		return nil, err
	}
	return &socketTransport{conn: conn, path: ""}, nil
}
