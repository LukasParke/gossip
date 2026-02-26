package transport

import (
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func TestMemoryPipe(t *testing.T) {
	t.Run("bidirectional", func(t *testing.T) {
		client, server := MemoryPipe()
		defer client.Close()
		defer server.Close()

		// Client -> Server
		want := []byte("hello from client")
		if _, err := client.Write(want); err != nil {
			t.Fatal(err)
		}
		got := make([]byte, len(want))
		if _, err := server.Read(got); err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("server read %q, want %q", got, want)
		}

		// Server -> Client
		want = []byte("hello from server")
		if _, err := server.Write(want); err != nil {
			t.Fatal(err)
		}
		got = make([]byte, len(want))
		if _, err := client.Read(got); err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("client read %q, want %q", got, want)
		}
	})

	t.Run("close propagates error", func(t *testing.T) {
		client, server := MemoryPipe()

		// Close one side
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}

		// Other side should get error on read
		buf := make([]byte, 16)
		_, err := server.Read(buf)
		if err == nil {
			t.Fatal("expected error on read after close, got nil")
		}
	})
}

func TestTCPTransport(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()

	var serverTransport Transport
	var acceptErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := ln.Accept()
		ln.Close()
		if err != nil {
			acceptErr = err
			return
		}
		serverTransport = TCP(conn)
	}()

	clientConn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	clientTransport := TCP(clientConn)

	wg.Wait()
	if acceptErr != nil {
		t.Fatal(acceptErr)
	}
	if serverTransport == nil {
		t.Fatal("server transport not set")
	}

	defer clientTransport.Close()
	defer serverTransport.Close()

	// Add timeout to prevent hangs
	done := make(chan struct{})
	go func() {
		// Server -> Client
		want := []byte("from server")
		if _, err := serverTransport.Write(want); err != nil {
			t.Error(err)
			return
		}
		got := make([]byte, len(want))
		if _, err := clientTransport.Read(got); err != nil {
			t.Error(err)
			return
		}
		if string(got) != string(want) {
			t.Errorf("client read %q, want %q", got, want)
			return
		}

		// Client -> Server
		want = []byte("from client")
		if _, err := clientTransport.Write(want); err != nil {
			t.Error(err)
			return
		}
		got = make([]byte, len(want))
		if _, err := serverTransport.Read(got); err != nil {
			t.Error(err)
			return
		}
		if string(got) != string(want) {
			t.Errorf("server read %q, want %q", got, want)
			return
		}
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(5 * time.Second):
		t.Fatal("test timed out after 5s")
	}

	// Close and verify other side gets error
	clientTransport.Close()
	buf := make([]byte, 16)
	_, err = serverTransport.Read(buf)
	if err == nil {
		t.Fatal("expected error on read after close, got nil")
	}
}

func TestWebSocketTransport(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()

	connCh := make(chan *websocket.Conn, 1)
	handler := websocket.Handler(func(ws *websocket.Conn) {
		connCh <- ws
		// Block until connection is done; echo messages back
		for {
			var msg []byte
			if err := websocket.Message.Receive(ws, &msg); err != nil {
				return
			}
			if err := websocket.Message.Send(ws, msg); err != nil {
				return
			}
		}
	})

	mux := http.NewServeMux()
	mux.Handle("/", handler)
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()

	time.Sleep(50 * time.Millisecond)

	wsURL := "ws://" + addr + "/"
	clientWS, err := websocket.Dial(wsURL, "", "http://localhost/")
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer clientWS.Close()

	// Wait for server-side connection
	var serverWS *websocket.Conn
	select {
	case serverWS = <-connCh:
		_ = serverWS
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server connection")
	}

	// Client -> Server -> Client (echo)
	want := []byte("hello ws")
	if err := websocket.Message.Send(clientWS, want); err != nil {
		t.Fatalf("client send: %v", err)
	}

	var got []byte
	if err := websocket.Message.Receive(clientWS, &got); err != nil {
		t.Fatalf("client receive echo: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("echo received %q, want %q", got, want)
	}
}

func TestWSTransportCloseIdempotent(t *testing.T) {
	// Use a real WebSocket connection so Close doesn't nil-deref
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()

	handler := websocket.Handler(func(ws *websocket.Conn) {
		buf := make([]byte, 1)
		ws.Read(buf)
	})
	mux := http.NewServeMux()
	mux.Handle("/", handler)
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()

	time.Sleep(50 * time.Millisecond)

	ws, err := websocket.Dial("ws://"+addr+"/", "", "http://localhost/")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	wst := &wsTransport{conn: ws, srv: srv, ln: ln}
	if err := wst.Close(); err != nil {
		t.Errorf("first close: %v", err)
	}
	// Second close should not panic (closeOnce)
	if err := wst.Close(); err != nil {
		t.Errorf("second close: %v", err)
	}
}

func TestIPCTransportType(t *testing.T) {
	// Verify ipcTransport satisfies the Transport interface at compile time
	var _ Transport = &ipcTransport{}
}

func TestWSTransportType(t *testing.T) {
	var _ Transport = &wsTransport{}
}
