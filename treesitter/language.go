package treesitter

import (
	"fmt"
	"path"
	"strings"
	"sync"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// Registry maps file extensions, filenames, patterns, and language IDs
// to tree-sitter languages.
type Registry struct {
	mu        sync.RWMutex
	languages map[string]*tree_sitter.Language // ext -> language
	matchers  []LanguageMatcher
}

// NewRegistry creates a new language registry from a config.
func NewRegistry(cfg Config) *Registry {
	r := &Registry{
		languages: make(map[string]*tree_sitter.Language, len(cfg.Languages)),
		matchers:  cfg.Matchers,
	}
	for ext, lang := range cfg.Languages {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		r.languages[ext] = lang
	}
	return r
}

// Register adds a language for a given file extension.
func (r *Registry) Register(ext string, lang *tree_sitter.Language) {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.languages[ext] = lang
}

// RegisterMatcher adds a LanguageMatcher to the registry. Matchers are
// evaluated in registration order; the first match wins over extension-based lookup.
func (r *Registry) RegisterMatcher(m LanguageMatcher) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.matchers = append(r.matchers, m)
}

// LanguageFor returns the tree-sitter language for a given URI or filename.
// It tries matchers first (in order), then falls back to extension-based lookup.
func (r *Registry) LanguageFor(uri string) (*tree_sitter.Language, error) {
	return r.LanguageForURI(uri, "")
}

// LanguageForURI returns the tree-sitter language for a given URI and optional
// languageID. It evaluates in this order:
//  1. Matchers: exact filename match
//  2. Matchers: languageID match
//  3. Matchers: glob pattern match
//  4. Matchers: extension match
//  5. Extension-based lookup from the Languages map
func (r *Registry) LanguageForURI(uri string, languageID string) (*tree_sitter.Language, error) {
	filename := path.Base(uri)
	ext := path.Ext(uri)

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Pass 1: exact filename match
	for _, m := range r.matchers {
		for _, fn := range m.Filenames {
			if fn == filename {
				return m.Language, nil
			}
		}
	}

	// Pass 2: languageID match
	if languageID != "" {
		for _, m := range r.matchers {
			if m.LanguageID != "" && m.LanguageID == languageID {
				return m.Language, nil
			}
		}
	}

	// Pass 3: glob pattern match
	for _, m := range r.matchers {
		if m.Pattern != "" {
			if matched, _ := path.Match(m.Pattern, uri); matched {
				return m.Language, nil
			}
			if matched, _ := path.Match(m.Pattern, filename); matched {
				return m.Language, nil
			}
		}
	}

	// Pass 4: matcher extension match
	if ext != "" {
		for _, m := range r.matchers {
			for _, mExt := range m.Extensions {
				normalized := mExt
				if !strings.HasPrefix(normalized, ".") {
					normalized = "." + normalized
				}
				if normalized == ext {
					return m.Language, nil
				}
			}
		}
	}

	// Pass 5: legacy extension map
	if ext != "" {
		if lang, ok := r.languages[ext]; ok {
			return lang, nil
		}
	}

	return nil, fmt.Errorf("no language registered for: %s", uri)
}

// HasLanguage returns whether a language is registered for the given URI.
func (r *Registry) HasLanguage(uri string) bool {
	lang, err := r.LanguageForURI(uri, "")
	return err == nil && lang != nil
}
