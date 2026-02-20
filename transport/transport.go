// Package transport provides pluggable I/O transports for LSP communication.
// Supported transports include stdio, TCP, Unix domain sockets, Windows
// named pipes, WebSocket, and Node.js IPC (VS Code extension host).
package transport

import "io"

// Transport provides a bidirectional byte stream for JSON-RPC communication.
// Each implementation wraps a specific communication mechanism (stdio, TCP, etc.)
// and exposes it as a simple reader/writer pair.
type Transport interface {
	io.ReadWriteCloser
}

// Func adapts a function that returns a Transport into a TransportProvider.
type Func func() (Transport, error)
