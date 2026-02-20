package transport

import (
	"io"
	"os"
)

type stdioTransport struct {
	in  io.ReadCloser
	out io.WriteCloser
}

// Stdio returns a Transport backed by os.Stdin and os.Stdout.
func Stdio() Transport {
	return &stdioTransport{in: os.Stdin, out: os.Stdout}
}

func (s *stdioTransport) Read(p []byte) (int, error)  { return s.in.Read(p) }
func (s *stdioTransport) Write(p []byte) (int, error) { return s.out.Write(p) }
func (s *stdioTransport) Close() error {
	s.in.Close()
	return s.out.Close()
}
