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

// ShowMessage sends a show-message notification to the client. The client
// typically displays this in a popup or status bar; typ controls severity.
func (c *ClientProxy) ShowMessage(ctx context.Context, typ protocol.MessageType, message string) error {
	return c.conn.Notify(ctx, protocol.MethodShowMessage, &protocol.ShowMessageParams{
		Type:    typ,
		Message: message,
	})
}

// ShowMessageRequest sends a show-message request and blocks until the user
// selects an action. Returns the chosen action, or nil if cancelled/dismissed.
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

// ApplyEdit requests the client to apply a workspace edit. Returns the client's
// response indicating whether the edit was applied and any failure message.
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

// Configuration requests workspace configuration values from the client. Items
// correspond to the scopeURIs in params; each item is the client's config for that scope.
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

// RegisterCapability dynamically registers a capability with the client. Use
// after initialization to enable features the server did not declare statically.
func (c *ClientProxy) RegisterCapability(ctx context.Context, params *protocol.RegistrationParams) error {
	_, err := c.conn.Call(ctx, protocol.MethodRegisterCapability, params)
	return err
}

// UnregisterCapability unregisters a previously registered capability.
func (c *ClientProxy) UnregisterCapability(ctx context.Context, params *protocol.UnregistrationParams) error {
	_, err := c.conn.Call(ctx, protocol.MethodUnregisterCapability, params)
	return err
}

// RefreshDiagnostics asks the client to re-request diagnostics. Use when
// diagnostics change outside of normal request/notification flow.
func (c *ClientProxy) RefreshDiagnostics(ctx context.Context) error {
	_, err := c.conn.Call(ctx, protocol.MethodDiagnosticRefresh, nil)
	return err
}

// RefreshInlayHints asks the client to re-request inlay hints.
func (c *ClientProxy) RefreshInlayHints(ctx context.Context) error {
	_, err := c.conn.Call(ctx, protocol.MethodInlayHintRefresh, nil)
	return err
}

// RefreshSemanticTokens asks the client to re-request semantic tokens.
func (c *ClientProxy) RefreshSemanticTokens(ctx context.Context) error {
	_, err := c.conn.Call(ctx, protocol.MethodSemanticTokensRefresh, nil)
	return err
}

// CreateWorkDoneProgress creates a work-done progress reporter. Use the token
// with ReportProgress to send begin/progress/end notifications.
func (c *ClientProxy) CreateWorkDoneProgress(ctx context.Context, token interface{}) error {
	_, err := c.conn.Call(ctx, protocol.MethodWorkDoneProgressCreate, &protocol.WorkDoneProgressCreateParams{
		Token: token,
	})
	return err
}

// ReportProgress sends a progress update for a work-done token. Value should
// be a WorkDoneProgressBegin, WorkDoneProgressReport, or WorkDoneProgressEnd.
func (c *ClientProxy) ReportProgress(ctx context.Context, token interface{}, value interface{}) error {
	return c.conn.Notify(ctx, protocol.MethodProgress, &protocol.ProgressParams{
		Token: token,
		Value: value,
	})
}
