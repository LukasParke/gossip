// Example gossip LSP server with TOML config and hot-reload.
package main

import (
	"fmt"
	"log"

	"github.com/LukasParke/gossip"
	"github.com/LukasParke/gossip/protocol"
)

type Config struct {
	MaxCompletions int      `toml:"max_completions"`
	Keywords       []string `toml:"keywords"`
}

func main() {
	defaults := Config{
		MaxCompletions: 10,
		Keywords:       []string{"func", "var", "const", "type", "import"},
	}

	s := gossip.NewServer("configurable-lsp", "0.1.0",
		gossip.WithConfig[Config](".configurable-lsp.toml", defaults),
	)

	gossip.OnConfigChange(s, func(ctx *gossip.Context, old, new_ *Config) {
		ctx.Client.LogMessage(ctx, protocol.Info, fmt.Sprintf("Config reloaded: %d keywords", len(new_.Keywords)))
	})

	s.OnCompletion(func(ctx *gossip.Context, p *protocol.CompletionParams) (*protocol.CompletionList, error) {
		cfg := gossip.Config[Config](ctx)
		if cfg == nil {
			cfg = &defaults
		}

		items := make([]protocol.CompletionItem, 0, len(cfg.Keywords))
		for i, kw := range cfg.Keywords {
			if i >= cfg.MaxCompletions {
				break
			}
			items = append(items, protocol.CompletionItem{
				Label: kw,
				Kind:  protocol.CompletionKindKeyword,
			})
		}

		return &protocol.CompletionList{Items: items}, nil
	})

	if err := gossip.Serve(s, gossip.FromArgs()); err != nil {
		log.Fatal(err)
	}
}
