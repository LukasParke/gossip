package jsonrpc

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// DefaultMaxMessageSize is the default maximum allowed message body size (32 MB).
const DefaultMaxMessageSize = 32 * 1024 * 1024

// Codec reads and writes Content-Length framed JSON-RPC messages
// as specified by the LSP base protocol.
type Codec struct {
	reader         *bufio.Reader
	writer         io.Writer
	wmu            sync.Mutex
	maxMessageSize int
	closer         io.Closer // underlying transport for Close
}

// NewCodec creates a new Content-Length framed codec over the given streams.
func NewCodec(r io.Reader, w io.Writer) *Codec {
	c := &Codec{
		reader:         bufio.NewReaderSize(r, 64*1024),
		writer:         w,
		maxMessageSize: DefaultMaxMessageSize,
	}
	if rc, ok := r.(io.Closer); ok {
		c.closer = rc
	}
	return c
}

// SetMaxMessageSize sets the maximum allowed Content-Length. Messages exceeding
// this limit are rejected with an error. Set to 0 to disable the limit.
func (c *Codec) SetMaxMessageSize(n int) {
	c.maxMessageSize = n
}

// Close closes the underlying transport if it implements io.Closer.
func (c *Codec) Close() error {
	if c.closer != nil {
		return c.closer.Close()
	}
	return nil
}

// Read reads a single Content-Length framed message from the stream.
func (c *Codec) Read() ([]byte, error) {
	contentLen := -1
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("reading header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		val := strings.TrimSpace(line[colon+1:])

		if strings.EqualFold(key, "Content-Length") {
			n, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length %q: %w", val, err)
			}
			contentLen = n
		}
	}

	if contentLen < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}
	if c.maxMessageSize > 0 && contentLen > c.maxMessageSize {
		return nil, fmt.Errorf("message size %d exceeds limit %d", contentLen, c.maxMessageSize)
	}

	body := make([]byte, contentLen)
	if _, err := io.ReadFull(c.reader, body); err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}
	return body, nil
}

// Write writes a Content-Length framed message to the stream.
func (c *Codec) Write(data []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Content-Length: %d\r\n\r\n", len(data))
	buf.Write(data)

	_, err := c.writer.Write(buf.Bytes())
	return err
}
