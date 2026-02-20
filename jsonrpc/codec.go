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

// Codec reads and writes Content-Length framed JSON-RPC messages
// as specified by the LSP base protocol.
type Codec struct {
	reader *bufio.Reader
	writer io.Writer
	wmu    sync.Mutex
}

// NewCodec creates a new Content-Length framed codec over the given streams.
func NewCodec(r io.Reader, w io.Writer) *Codec {
	return &Codec{
		reader: bufio.NewReaderSize(r, 64*1024),
		writer: w,
	}
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
