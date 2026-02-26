// Package jsonrpc implements a bidirectional JSON-RPC 2.0 connection over
// Content-Length framed streams, as specified by the LSP base protocol.
package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

// Handler processes an incoming JSON-RPC request or notification.
type Handler func(ctx context.Context, method string, params RawMessage) (result interface{}, err error)

// NotificationHandler processes an incoming JSON-RPC notification.
type NotificationHandler func(ctx context.Context, method string, params RawMessage)

// Conn is a bidirectional JSON-RPC 2.0 connection.
type Conn struct {
	codec   *Codec
	handler Handler
	notif   NotificationHandler

	pending   sync.Map // id -> chan *Response
	nextID    atomic.Int64
	closeOnce sync.Once
	done      chan struct{}
}

// NewConn creates a new JSON-RPC connection using the given codec, request
// handler, and notification handler.
func NewConn(codec *Codec, handler Handler, notif NotificationHandler) *Conn {
	return &Conn{
		codec:   codec,
		handler: handler,
		notif:   notif,
		done:    make(chan struct{}),
	}
}

type readResult struct {
	data []byte
	err  error
}

// Run reads messages from the connection until it is closed or an error occurs.
func (c *Conn) Run(ctx context.Context) error {
	incoming := make(chan readResult, 1)
	go func() {
		for {
			data, err := c.codec.Read()
			select {
			case incoming <- readResult{data, err}:
			case <-c.done:
				return
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.done:
			return nil
		case res := <-incoming:
			if res.err != nil {
				select {
				case <-c.done:
					return nil
				default:
					return fmt.Errorf("reading message: %w", res.err)
				}
			}
			msg, err := DecodeMessage(res.data)
			if err != nil {
				c.sendParseError(res.data, err)
				continue
			}
			switch m := msg.(type) {
			case *Request:
				go c.handleRequest(ctx, m)
			case *Notification:
				go c.handleNotification(ctx, m)
			case *Response:
				c.handleResponse(m)
			}
		}
	}
}

// sendParseError attempts to respond with a parse error if the raw data
// looks like it might contain a request ID.
func (c *Conn) sendParseError(data []byte, parseErr error) {
	var probe struct {
		ID *ID `json:"id,omitempty"`
	}
	if json.Unmarshal(data, &probe) == nil && probe.ID != nil && probe.ID.IsValid() {
		resp := NewResponse(*probe.ID, nil, &Error{Code: CodeParseError, Message: parseErr.Error()})
		if respData, err := json.Marshal(resp); err == nil {
			_ = c.codec.Write(respData)
		}
	}
}

func (c *Conn) handleRequest(ctx context.Context, req *Request) {
	result, err := c.handler(ctx, req.Method, req.Params)
	resp := NewResponse(req.ID, result, err)
	data, merr := json.Marshal(resp)
	if merr != nil {
		return
	}
	_ = c.codec.Write(data)
}

func (c *Conn) handleNotification(ctx context.Context, notif *Notification) {
	if c.notif != nil {
		c.notif(ctx, notif.Method, notif.Params)
	} else if c.handler != nil {
		c.handler(ctx, notif.Method, notif.Params)
	}
}

func (c *Conn) handleResponse(resp *Response) {
	if ch, ok := c.pending.LoadAndDelete(formatID(resp.ID)); ok {
		ch.(chan *Response) <- resp
	}
}

// Call sends a request and waits for a response.
func (c *Conn) Call(ctx context.Context, method string, params interface{}) (*Response, error) {
	id := IntID(c.nextID.Add(1))
	paramsData, err := marshalParams(params)
	if err != nil {
		return nil, err
	}

	req := &Request{
		JSONRPC: Version,
		ID:      id,
		Method:  method,
		Params:  paramsData,
	}

	ch := make(chan *Response, 1)
	c.pending.Store(formatID(id), ch)
	defer c.pending.Delete(formatID(id))

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if err := c.codec.Write(data); err != nil {
		return nil, err
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, fmt.Errorf("connection closed")
	}
}

// Notify sends a notification (no response expected).
func (c *Conn) Notify(ctx context.Context, method string, params interface{}) error {
	paramsData, err := marshalParams(params)
	if err != nil {
		return err
	}

	notif := &Notification{
		JSONRPC: Version,
		Method:  method,
		Params:  paramsData,
	}

	data, err := json.Marshal(notif)
	if err != nil {
		return err
	}
	return c.codec.Write(data)
}

// Close terminates the connection and closes the underlying codec.
func (c *Conn) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		c.codec.Close()
	})
}

func marshalParams(v interface{}) (RawMessage, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func formatID(id ID) string {
	switch v := id.Value().(type) {
	case int64:
		return fmt.Sprintf("n:%d", v)
	case string:
		return fmt.Sprintf("s:%s", v)
	default:
		return "null"
	}
}
