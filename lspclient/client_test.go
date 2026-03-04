package lspclient

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/LukasParke/gossip/jsonrpc"
)

func TestClient_handleConfiguration(t *testing.T) {
	c := &Client{
		opts: ClientOptions{
			Settings: map[string]any{
				"yaml": map[string]any{
					"validate":   true,
					"completion": false,
				},
				"json": map[string]any{
					"validate": map[string]any{"enable": true},
				},
			},
		},
	}

	params, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{"section": "yaml"},
			{"section": "json"},
			{"section": "unknown"},
		},
	})

	result, err := c.handleConfiguration(jsonrpc.RawMessage(params))
	if err != nil {
		t.Fatalf("handleConfiguration error: %v", err)
	}

	results, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	yamlCfg, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map for yaml section, got %T", results[0])
	}
	if yamlCfg["validate"] != true {
		t.Error("expected yaml.validate = true")
	}

	if results[2] != nil {
		t.Error("expected nil for unknown section")
	}
}

func TestClient_handleRequest_unknownMethod(t *testing.T) {
	c := NewClient(ClientOptions{})

	_, err := c.handleRequest(context.Background(), "unknown/method", nil)
	if err == nil {
		t.Fatal("expected error for unknown method")
	}
	rpcErr, ok := err.(*jsonrpc.Error)
	if !ok {
		t.Fatalf("expected *jsonrpc.Error, got %T", err)
	}
	if rpcErr.Code != jsonrpc.CodeMethodNotFound {
		t.Errorf("expected code %d, got %d", jsonrpc.CodeMethodNotFound, rpcErr.Code)
	}
}

func TestClient_DidOpen_NotRunning(t *testing.T) {
	c := NewClient(ClientOptions{})
	// Should not panic or error when not running
	if err := c.DidOpen(context.Background(), nil); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestClient_DidChange_NotRunning(t *testing.T) {
	c := NewClient(ClientOptions{})
	if err := c.DidChange(context.Background(), nil); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestClient_DidClose_NotRunning(t *testing.T) {
	c := NewClient(ClientOptions{})
	if err := c.DidClose(context.Background(), nil); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestClient_Stop_NotRunning(t *testing.T) {
	c := NewClient(ClientOptions{})
	if err := c.Stop(context.Background()); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}
