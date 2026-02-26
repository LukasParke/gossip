package transport

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"golang.org/x/net/websocket"
)

// ListenWebSocket starts an HTTP server with WebSocket upgrade on the given
// address and returns the first WebSocket connection as a transport.
// Used by Monaco, Theia, and other web-based editors.
func ListenWebSocket(addr string) (Transport, error) {
	connCh := make(chan *websocket.Conn, 1)
	errCh := make(chan error, 1)

	handler := websocket.Handler(func(ws *websocket.Conn) {
		connCh <- ws
		select {} // Block until the connection is closed by the transport
	})

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	srv := &http.Server{Handler: handler}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("websocket server error: %w", err)
		}
	}()

	select {
	case ws := <-connCh:
		return &wsTransport{conn: ws, srv: srv, ln: ln}, nil
	case err := <-errCh:
		ln.Close()
		return nil, err
	}
}

type wsTransport struct {
	conn *websocket.Conn
	srv  *http.Server
	ln   net.Listener

	closeOnce sync.Once
}

func (w *wsTransport) Read(p []byte) (int, error) {
	var msg []byte
	err := websocket.Message.Receive(w.conn, &msg)
	if err != nil {
		return 0, err
	}
	n := copy(p, msg)
	if n < len(msg) {
		return n, io.ErrShortBuffer
	}
	return n, nil
}

func (w *wsTransport) Write(p []byte) (int, error) {
	err := websocket.Message.Send(w.conn, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *wsTransport) Close() error {
	var err error
	w.closeOnce.Do(func() {
		w.conn.Close()
		if w.srv != nil {
			w.srv.Close()
		}
	})
	return err
}
