package treesitter

import (
	"fmt"
	"path"
	"strings"
	"sync"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// Registry maps URIs and filenames to tree-sitter languages via extensions,
// exact filenames, glob patterns, and LSP language IDs. It is safe for
// concurrent use. Matchers are evaluated before the extension map.
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

// Register adds a language for a file extension (e.g., ".go", "go"). Leading
// dot is added if missing. Overwrites any existing registration for that ext.
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

// LanguageFor returns the tree-sitter language for a URI or filename. It calls
// LanguageForURI with an empty languageID. Use LanguageForURI when the LSP
// languageId is available for better matching.
func (r *Registry) LanguageFor(uri string) (*tree_sitter.Language, error) {
	return r.LanguageForURI(uri, "")
}

// LanguageForURI returns the tree-sitter language for a URI and optional LSP
// languageId. Evaluation order: (1) matcher exact filename, (2) matcher
// languageID, (3) matcher glob pattern, (4) matcher extension, (5) Config
// Languages extension map. Returns an error if no match is found.
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

// HasLanguage reports whether a language is registered for the URI (same
// lookup logic as LanguageForURI, but without returning the language).
func (r *Registry) HasLanguage(uri string) bool {
	lang, err := r.LanguageForURI(uri, "")
	return err == nil && lang != nil
}
