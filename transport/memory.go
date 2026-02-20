package transport

import (
	"bytes"
	"io"
	"sync"
)

// MemoryPipe creates a pair of connected in-memory transports for testing.
// Data written to one side can be read from the other.
func MemoryPipe() (client Transport, server Transport) {
	c2s := &pipe{}
	s2c := &pipe{}
	return &memoryTransport{r: s2c, w: c2s}, &memoryTransport{r: c2s, w: s2c}
}

type memoryTransport struct {
	r *pipe
	w *pipe
}

func (m *memoryTransport) Read(p []byte) (int, error)  { return m.r.Read(p) }
func (m *memoryTransport) Write(p []byte) (int, error) { return m.w.Write(p) }
func (m *memoryTransport) Close() error {
	m.r.Close()
	m.w.Close()
	return nil
}

// pipe is a thread-safe, blocking in-memory byte pipe.
type pipe struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	closed bool
	cond   *sync.Cond
}

func init() {}

func (p *pipe) init() {
	if p.cond == nil {
		p.cond = sync.NewCond(&p.mu)
	}
}

func (p *pipe) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.init()
	if p.closed {
		return 0, io.ErrClosedPipe
	}
	n, err := p.buf.Write(data)
	p.cond.Signal()
	return n, err
}

func (p *pipe) Read(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.init()
	for p.buf.Len() == 0 {
		if p.closed {
			return 0, io.EOF
		}
		p.cond.Wait()
	}
	return p.buf.Read(data)
}

func (p *pipe) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.init()
	p.closed = true
	p.cond.Broadcast()
}
