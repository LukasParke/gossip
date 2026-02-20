package gossip

import (
	"context"
	"encoding/json"

	"github.com/LukasParke/gossip/jsonrpc"
	"github.com/LukasParke/gossip/protocol"
)

// ClientProxy sends requests and notifications from server to client.
type ClientProxy struct {
	conn *jsonrpc.Conn
}

func newClientProxy(conn *jsonrpc.Conn) *ClientProxy {
	return &ClientProxy{conn: conn}
}

// PublishDiagnostics sends diagnostics for a document to the client.
func (c *ClientProxy) PublishDiagnostics(ctx context.Context, params *protocol.PublishDiagnosticsParams) error {
	return c.conn.Notify(ctx, protocol.MethodPublishDiagnostics, params)
}

// LogMessage sends a log message to the client.
func (c *ClientProxy) LogMessage(ctx context.Context, typ protocol.MessageType, message string) error {
	return c.conn.Notify(ctx, protocol.MethodLogMessage, &protocol.LogMessageParams{
		Type:    typ,
		Message: message,
	})
}

// ShowMessage sends a show message notification to the client.
func (c *ClientProxy) ShowMessage(ctx context.Context, typ protocol.MessageType, message string) error {
	return c.conn.Notify(ctx, protocol.MethodShowMessage, &protocol.ShowMessageParams{
		Type:    typ,
		Message: message,
	})
}

// ShowMessageRequest sends a show message request and waits for the user to pick an action.
func (c *ClientProxy) ShowMessageRequest(ctx context.Context, params *protocol.ShowMessageRequestParams) (*protocol.MessageActionItem, error) {
	resp, err := c.conn.Call(ctx, protocol.MethodShowMessageRequest, params)
	if err != nil {
		return nil, err
	}
	if resp.Result == nil {
		return nil, nil
	}
	var item protocol.MessageActionItem
	if err := json.Unmarshal(resp.Result, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// ApplyEdit requests the client to apply a workspace edit.
func (c *ClientProxy) ApplyEdit(ctx context.Context, params *protocol.ApplyWorkspaceEditParams) (*protocol.ApplyWorkspaceEditResponse, error) {
	resp, err := c.conn.Call(ctx, protocol.MethodApplyEdit, params)
	if err != nil {
		return nil, err
	}
	var result protocol.ApplyWorkspaceEditResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Configuration requests configuration values from the client.
func (c *ClientProxy) Configuration(ctx context.Context, params *protocol.ConfigurationParams) ([]json.RawMessage, error) {
	resp, err := c.conn.Call(ctx, protocol.MethodWorkspaceConfiguration, params)
	if err != nil {
		return nil, err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(resp.Result, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// RegisterCapability dynamically registers a capability with the client.
func (c *ClientProxy) RegisterCapability(ctx context.Context, params *protocol.RegistrationParams) error {
	_, err := c.conn.Call(ctx, protocol.MethodRegisterCapability, params)
	return err
}

// UnregisterCapability dynamically unregisters a capability with the client.
func (c *ClientProxy) UnregisterCapability(ctx context.Context, params *protocol.UnregistrationParams) error {
	_, err := c.conn.Call(ctx, protocol.MethodUnregisterCapability, params)
	return err
}

// RefreshDiagnostics asks the client to re-pull diagnostics.
func (c *ClientProxy) RefreshDiagnostics(ctx context.Context) error {
	_, err := c.conn.Call(ctx, protocol.MethodDiagnosticRefresh, nil)
	return err
}

// RefreshInlayHints asks the client to re-pull inlay hints.
func (c *ClientProxy) RefreshInlayHints(ctx context.Context) error {
	_, err := c.conn.Call(ctx, protocol.MethodInlayHintRefresh, nil)
	return err
}

// RefreshSemanticTokens asks the client to re-pull semantic tokens.
func (c *ClientProxy) RefreshSemanticTokens(ctx context.Context) error {
	_, err := c.conn.Call(ctx, protocol.MethodSemanticTokensRefresh, nil)
	return err
}
