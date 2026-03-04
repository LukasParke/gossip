// Package lspclient provides a reusable LSP client for managing child
// language server subprocesses. It handles process lifecycle, LSP
// initialization, document synchronization, and diagnostic collection.
package lspclient

import (
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// ProcessTransport wraps an exec.Cmd, using its stdin pipe for writing and
// stdout pipe for reading. It implements io.ReadWriteCloser so it can be
// passed directly to jsonrpc.NewCodec.
type ProcessTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	closeOnce sync.Once
	closeErr  error
}

// NewProcessTransport creates a transport backed by a child process.
// The process is started immediately. Use Close to terminate it.
func NewProcessTransport(name string, args ...string) (*ProcessTransport, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = nil // discard child stderr; callers can override before Start if needed

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("lspclient: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("lspclient: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("lspclient: start %s: %w", name, err)
	}

	return &ProcessTransport{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
	}, nil
}

func (t *ProcessTransport) Read(p []byte) (int, error)  { return t.stdout.Read(p) }
func (t *ProcessTransport) Write(p []byte) (int, error) { return t.stdin.Write(p) }

// Close terminates the child process and releases resources.
func (t *ProcessTransport) Close() error {
	t.closeOnce.Do(func() {
		t.stdin.Close()
		t.stdout.Close()
		// Kill if still running; ignore errors (process may have exited already).
		if t.cmd.Process != nil {
			_ = t.cmd.Process.Kill()
		}
		t.closeErr = t.cmd.Wait()
	})
	return t.closeErr
}

// Process returns the underlying exec.Cmd for inspection (e.g. PID).
func (t *ProcessTransport) Process() *exec.Cmd { return t.cmd }
