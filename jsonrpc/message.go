package jsonrpc

import "encoding/json"

const Version = "2.0"

// RawMessage is a raw JSON value that delays unmarshaling.
type RawMessage = json.RawMessage

// Message is the common interface for all JSON-RPC 2.0 messages.
type Message interface {
	isJSONRPC()
}

// Request is a JSON-RPC 2.0 request (expects a response).
type Request struct {
	JSONRPC string     `json:"jsonrpc"`
	ID      ID         `json:"id"`
	Method  string     `json:"method"`
	Params  RawMessage `json:"params,omitempty"`
}

func (Request) isJSONRPC() {}

// Notification is a JSON-RPC 2.0 notification (no response expected).
type Notification struct {
	JSONRPC string     `json:"jsonrpc"`
	Method  string     `json:"method"`
	Params  RawMessage `json:"params,omitempty"`
}

func (Notification) isJSONRPC() {}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string     `json:"jsonrpc"`
	ID      ID         `json:"id"`
	Result  RawMessage `json:"result,omitempty"`
	Error   *Error     `json:"error,omitempty"`
}

func (Response) isJSONRPC() {}

// Error represents a JSON-RPC 2.0 error object.
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *Error) Error() string { return e.Message }

// Standard JSON-RPC 2.0 error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// LSP-specific error codes.
const (
	CodeServerNotInitialized = -32002
	CodeRequestCancelled     = -32800
	CodeContentModified      = -32801
)

// ID represents a JSON-RPC 2.0 request ID (int or string).
type ID struct {
	value interface{}
}

// IntID creates an integer-valued JSON-RPC request ID.
func IntID(v int64) ID { return ID{value: v} }

// StringID creates a string-valued JSON-RPC request ID.
func StringID(v string) ID { return ID{value: v} }

func (id ID) IsValid() bool { return id.value != nil }
func (id ID) Value() interface{} { return id.value }

func (id ID) MarshalJSON() ([]byte, error) {
	if id.value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(id.value)
}

func (id *ID) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		id.value = nil
		return nil
	}
	var n int64
	if err := json.Unmarshal(data, &n); err == nil {
		id.value = n
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		id.value = s
		return nil
	}
	return &Error{Code: CodeInvalidRequest, Message: "id must be a number, string, or null"}
}

// DecodeMessage parses a raw JSON blob into a Request, Notification, or Response.
func DecodeMessage(data []byte) (Message, error) {
	var raw struct {
		JSONRPC string     `json:"jsonrpc"`
		ID      *ID        `json:"id,omitempty"`
		Method  string     `json:"method,omitempty"`
		Result  RawMessage `json:"result,omitempty"`
		Error   *Error     `json:"error,omitempty"`
		Params  RawMessage `json:"params,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, &Error{Code: CodeParseError, Message: "failed to parse JSON-RPC message"}
	}

	if raw.Method != "" {
		if raw.ID != nil && raw.ID.IsValid() {
			return &Request{
				JSONRPC: raw.JSONRPC,
				ID:      *raw.ID,
				Method:  raw.Method,
				Params:  raw.Params,
			}, nil
		}
		return &Notification{
			JSONRPC: raw.JSONRPC,
			Method:  raw.Method,
			Params:  raw.Params,
		}, nil
	}

	id := ID{}
	if raw.ID != nil {
		id = *raw.ID
	}
	return &Response{
		JSONRPC: raw.JSONRPC,
		ID:      id,
		Result:  raw.Result,
		Error:   raw.Error,
	}, nil
}

// NewResponse creates a JSON-RPC response for the given request ID. If err is
// non-nil, the response contains an error; otherwise the result is marshaled.
func NewResponse(id ID, result interface{}, err error) *Response {
	resp := &Response{
		JSONRPC: Version,
		ID:      id,
	}
	if err != nil {
		if rpcErr, ok := err.(*Error); ok {
			resp.Error = rpcErr
		} else {
			resp.Error = &Error{Code: CodeInternalError, Message: err.Error()}
		}
		return resp
	}
	if result != nil {
		data, merr := json.Marshal(result)
		if merr != nil {
			resp.Error = &Error{Code: CodeInternalError, Message: merr.Error()}
			return resp
		}
		resp.Result = data
	} else {
		resp.Result = RawMessage("null")
	}
	return resp
}
