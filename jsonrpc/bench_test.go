package jsonrpc

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

func BenchmarkCodecWrite(b *testing.B) {
	for _, size := range []int{64, 1024, 16384} {
		msg := []byte(strings.Repeat("x", size))
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			codec := NewCodec(strings.NewReader(""), io.Discard)
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := codec.Write(msg); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkCodecRead(b *testing.B) {
	for _, size := range []int{64, 1024, 16384} {
		body := strings.Repeat("x", size)
		frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", size, body)

		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			data := strings.Repeat(frame, b.N)
			codec := NewCodec(strings.NewReader(data), io.Discard)
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := codec.Read(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkCodecRoundtrip(b *testing.B) {
	msg := []byte(`{"jsonrpc":"2.0","id":1,"method":"textDocument/hover","params":{"textDocument":{"uri":"file:///test.go"},"position":{"line":10,"character":5}}}`)
	b.SetBytes(int64(len(msg)))

	pr, pw := io.Pipe()
	codec := NewCodec(pr, pw)

	go func() {
		for i := 0; i < b.N; i++ {
			if err := codec.Write(msg); err != nil {
				return
			}
		}
		pw.Close()
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := codec.Read(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFrameEncode(b *testing.B) {
	msg := []byte(`{"jsonrpc":"2.0","id":1,"method":"test"}`)
	b.SetBytes(int64(len(msg)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "Content-Length: %d\r\n\r\n", len(msg))
		buf.Write(msg)
	}
}
