// Package gossip is a batteries-included Go framework for building Language
// Server Protocol (LSP) servers. It provides functional handler registration,
// auto-detected capabilities, composable middleware, built-in document
// management with tree-sitter integration, typed config with hot-reload,
// and first-class testing utilities.
//
// A minimal server needs only a few lines:
//
//	s := gossip.NewServer("my-lang", "0.1.0")
//	s.OnHover(myHoverHandler)
//	gossip.Serve(s, gossip.WithStdio())
//
// See the examples/ directory for progressively more complete servers.
package gossip
