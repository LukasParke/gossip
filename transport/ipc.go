package transport

import "os"

// NodeIPC creates a transport for Node.js IPC communication,
// as used by the VS Code extension host. Node.js IPC uses
// file descriptor 3 for reading and stdout (fd 1) for writing
// when the language server is spawned as a child process.
func NodeIPC() Transport {
	// Node.js IPC: the parent (VS Code) sends on the child's fd 3,
	// and reads from the child's stdout.
	reader := os.NewFile(3, "node-ipc-in")
	writer := os.Stdout
	return &ipcTransport{reader: reader, writer: writer}
}

type ipcTransport struct {
	reader *os.File
	writer *os.File
}

func (t *ipcTransport) Read(p []byte) (int, error)  { return t.reader.Read(p) }
func (t *ipcTransport) Write(p []byte) (int, error) { return t.writer.Write(p) }

// Close closes the IPC reader (fd 3). The writer (stdout) is intentionally
// left open because it is a shared resource owned by the process.
func (t *ipcTransport) Close() error {
	return t.reader.Close()
}
