package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

// --- Codec tests ---

func TestCodecReadWrite(t *testing.T) {
	pr, pw := io.Pipe()
	codec := NewCodec(pr, pw)
	defer codec.Close()

	msg := []byte(`{"jsonrpc":"2.0","id":1,"method":"test"}`)
	go func() {
		if err := codec.Write(msg); err != nil {
			t.Errorf("Write failed: %v", err)
		}
		pw.Close()
	}()

	got, err := codec.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(got) != string(msg) {
		t.Errorf("got %q, want %q", got, msg)
	}
}

func TestCodecReadMissingContentLength(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		pw.Write([]byte("Other-Header: value\r\n\r\n"))
		pw.Close()
	}()

	codec := NewCodec(pr, nil)
	_, err := codec.Read()
	if err == nil {
		t.Fatal("expected error for missing Content-Length, got nil")
	}
	if !strings.Contains(err.Error(), "missing Content-Length") {
		t.Errorf("expected 'missing Content-Length' in error, got: %v", err)
	}
}

func TestCodecMaxMessageSize(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		pw.Write([]byte("Content-Length: 100\r\n\r\n"))
		pw.Write(make([]byte, 100))
		pw.Close()
	}()

	codec := NewCodec(pr, nil)
	codec.SetMaxMessageSize(10)
	_, err := codec.Read()
	if err == nil {
		t.Fatal("expected error for message exceeding limit, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds limit") {
		t.Errorf("expected 'exceeds limit' in error, got: %v", err)
	}
}

// --- Message encode/decode tests ---

func TestDecodeRequest(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":1,"method":"foo","params":[1,2]}`)
	msg, err := DecodeMessage(data)
	if err != nil {
		t.Fatalf("DecodeMessage failed: %v", err)
	}
	req, ok := msg.(*Request)
	if !ok {
		t.Fatalf("expected *Request, got %T", msg)
	}
	if req.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q, want 2.0", req.JSONRPC)
	}
	if req.Method != "foo" {
		t.Errorf("Method = %q, want foo", req.Method)
	}
	if req.ID.Value() != int64(1) {
		t.Errorf("ID = %v, want 1", req.ID.Value())
	}
	if string(req.Params) != "[1,2]" {
		t.Errorf("Params = %q, want [1,2]", req.Params)
	}
}

func TestDecodeNotification(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","method":"notify","params":{}}`)
	msg, err := DecodeMessage(data)
	if err != nil {
		t.Fatalf("DecodeMessage failed: %v", err)
	}
	notif, ok := msg.(*Notification)
	if !ok {
		t.Fatalf("expected *Notification, got %T", msg)
	}
	if notif.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q, want 2.0", notif.JSONRPC)
	}
	if notif.Method != "notify" {
		t.Errorf("Method = %q, want notify", notif.Method)
	}
}

func TestDecodeResponse(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":2,"result":{"ok":true}}`)
	msg, err := DecodeMessage(data)
	if err != nil {
		t.Fatalf("DecodeMessage failed: %v", err)
	}
	resp, ok := msg.(*Response)
	if !ok {
		t.Fatalf("expected *Response, got %T", msg)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q, want 2.0", resp.JSONRPC)
	}
	if resp.ID.Value() != int64(2) {
		t.Errorf("ID = %v, want 2", resp.ID.Value())
	}
	if string(resp.Result) != `{"ok":true}` {
		t.Errorf("Result = %q, want {\"ok\":true}", resp.Result)
	}
	if resp.Error != nil {
		t.Errorf("Error = %v, want nil", resp.Error)
	}
}

func TestDecodeInvalid(t *testing.T) {
	_, err := DecodeMessage([]byte("invalid json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	var rpcErr *Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if rpcErr.Code != CodeParseError {
		t.Errorf("Code = %d, want CodeParseError (%d)", rpcErr.Code, CodeParseError)
	}
}

// --- ID tests ---

func TestIntID(t *testing.T) {
	id := IntID(42)
	if !id.IsValid() {
		t.Error("IntID(42) should be valid")
	}
	if id.Value() != int64(42) {
		t.Errorf("Value() = %v, want 42", id.Value())
	}
}

func TestStringID(t *testing.T) {
	id := StringID("abc")
	if !id.IsValid() {
		t.Error("StringID(\"abc\") should be valid")
	}
	if id.Value() != "abc" {
		t.Errorf("Value() = %v, want abc", id.Value())
	}
}

func TestIDMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		id   ID
		json string
	}{
		{"int", IntID(1), "1"},
		{"string", StringID("x"), `"x"`},
		{"null", ID{}, "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.id)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(data) != tt.json {
				t.Errorf("Marshal = %s, want %s", data, tt.json)
			}

			var out ID
			if err := json.Unmarshal(data, &out); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			// Compare values; for null, both should be invalid
			if tt.id.IsValid() != out.IsValid() {
				t.Errorf("IsValid mismatch: want %v", tt.id.IsValid())
			}
			if tt.id.IsValid() && tt.id.Value() != out.Value() {
				t.Errorf("Value mismatch: got %v, want %v", out.Value(), tt.id.Value())
			}
		})
	}
}

// --- NewResponse tests ---

func TestNewResponseSuccess(t *testing.T) {
	id := IntID(1)
	result := map[string]int{"a": 1}
	resp := NewResponse(id, result, nil)

	if resp.JSONRPC != Version {
		t.Errorf("JSONRPC = %q, want 2.0", resp.JSONRPC)
	}
	if resp.ID.Value() != int64(1) {
		t.Errorf("ID = %v, want 1", resp.ID.Value())
	}
	if resp.Error != nil {
		t.Errorf("Error = %v, want nil", resp.Error)
	}
	var got map[string]int
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}
	if got["a"] != 1 {
		t.Errorf("result[\"a\"] = %d, want 1", got["a"])
	}
}

func TestNewResponseError(t *testing.T) {
	id := IntID(2)
	resp := NewResponse(id, nil, &Error{Code: CodeMethodNotFound, Message: "not found"})

	if resp.Error == nil {
		t.Fatal("expected Error to be set")
	}
	if resp.Error.Code != CodeMethodNotFound {
		t.Errorf("Error.Code = %d, want %d", resp.Error.Code, CodeMethodNotFound)
	}
	if resp.Error.Message != "not found" {
		t.Errorf("Error.Message = %q, want not found", resp.Error.Message)
	}
}

func TestNewResponseNil(t *testing.T) {
	id := IntID(3)
	resp := NewResponse(id, nil, nil)

	if resp.Error != nil {
		t.Errorf("Error = %v, want nil", resp.Error)
	}
	if string(resp.Result) != "null" {
		t.Errorf("Result = %q, want null", resp.Result)
	}
}

// --- Conn tests ---

func TestConnCallAndResponse(t *testing.T) {
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	serverCodec := NewCodec(sr, sw)
	clientCodec := NewCodec(cr, cw)

	// Server: echo back method as result
	handler := func(ctx context.Context, method string, params RawMessage) (interface{}, error) {
		return method, nil
	}

	server := NewConn(serverCodec, handler, nil)
	client := NewConn(clientCodec, nil, nil)

	ctx := context.Background()
	serverDone := make(chan struct{})
	clientDone := make(chan struct{})
	go func() {
		_ = server.Run(ctx)
		close(serverDone)
	}()
	go func() {
		_ = client.Run(ctx)
		close(clientDone)
	}()

	resp, err := client.Call(ctx, "echo", nil)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("response error: %v", resp.Error)
	}
	var result string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}
	if result != "echo" {
		t.Errorf("result = %q, want echo", result)
	}

	client.Close()
	server.Close()
	<-serverDone
	<-clientDone
}

func TestConnNotify(t *testing.T) {
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	serverCodec := NewCodec(sr, sw)
	clientCodec := NewCodec(cr, cw)

	var notifCalled sync.WaitGroup
	notifCalled.Add(1)
	var gotMethod string
	var gotParams RawMessage
	notif := func(ctx context.Context, method string, params RawMessage) {
		gotMethod = method
		gotParams = params
		notifCalled.Done()
	}

	server := NewConn(serverCodec, nil, notif)
	client := NewConn(clientCodec, nil, nil)

	ctx := context.Background()
	go server.Run(ctx)
	go client.Run(ctx)

	if err := client.Notify(ctx, "testNotify", []int{1, 2}); err != nil {
		t.Fatalf("Notify failed: %v", err)
	}

	notifCalled.Wait()
	if gotMethod != "testNotify" {
		t.Errorf("gotMethod = %q, want testNotify", gotMethod)
	}
	if string(gotParams) != "[1,2]" {
		t.Errorf("gotParams = %q, want [1,2]", gotParams)
	}

	client.Close()
	server.Close()
}

func TestConnClose(t *testing.T) {
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	serverCodec := NewCodec(sr, sw)
	clientCodec := NewCodec(cr, cw)

	server := NewConn(serverCodec, func(ctx context.Context, method string, params RawMessage) (interface{}, error) {
		return method, nil
	}, nil)
	client := NewConn(clientCodec, nil, nil)

	ctx := context.Background()
	runDone := make(chan error, 1)
	go func() {
		runDone <- server.Run(ctx)
	}()
	go client.Run(ctx)

	client.Close()
	server.Close()

	err := <-runDone
	if err != nil {
		t.Errorf("Run returned error: %v, want nil", err)
	}
}

func TestConnConcurrent(t *testing.T) {
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	serverCodec := NewCodec(sr, sw)
	clientCodec := NewCodec(cr, cw)

	handler := func(ctx context.Context, method string, params RawMessage) (interface{}, error) {
		return method, nil
	}

	server := NewConn(serverCodec, handler, nil)
	client := NewConn(clientCodec, nil, nil)

	ctx := context.Background()
	go server.Run(ctx)
	go client.Run(ctx)

	var wg sync.WaitGroup
	const n = 10
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			method := "call"
			resp, err := client.Call(ctx, method, nil)
			if err != nil {
				t.Errorf("Call %d failed: %v", idx, err)
				return
			}
			if resp.Error != nil {
				t.Errorf("Call %d got error: %v", idx, resp.Error)
				return
			}
			var result string
			if err := json.Unmarshal(resp.Result, &result); err != nil {
				t.Errorf("Call %d Unmarshal: %v", idx, err)
				return
			}
			if result != method {
				t.Errorf("Call %d result = %q, want %q", idx, result, method)
			}
		}(i)
	}
	wg.Wait()

	client.Close()
	server.Close()
}
