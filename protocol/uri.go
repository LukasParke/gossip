package protocol

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

// NormalizeURI canonicalizes a DocumentURI so that different encodings of the
// same file path produce identical strings. This is modeled after gopls's
// ParseDocumentURI and terraform-ls's MustParseURI.
//
// For file:// URIs it cleans the path (resolving . and ..), strips host,
// query, and fragment components, and re-encodes to a canonical form.
// Non-file URIs are returned unchanged.
func NormalizeURI(uri DocumentURI) DocumentURI {
	s := string(uri)
	if s == "" {
		return uri
	}

	u, err := url.Parse(s)
	if err != nil {
		return uri
	}

	if u.Scheme != "file" {
		return uri
	}

	path := u.Path
	if path == "" {
		return uri
	}

	// On Windows, file URIs may have paths like /C:/foo; filepath.FromSlash
	// and filepath.Clean handle this correctly.
	cleaned := filepath.Clean(filepath.FromSlash(path))

	// Normalize Windows drive letters to uppercase (C: not c:).
	if runtime.GOOS == "windows" && len(cleaned) >= 2 && cleaned[1] == ':' {
		cleaned = strings.ToUpper(cleaned[:1]) + cleaned[1:]
	}

	// Reconstruct a canonical file URI.
	result := &url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(cleaned),
	}

	return DocumentURI(result.String())
}

// URIToPath extracts a cleaned filesystem path from a file:// URI. For
// non-file URIs or parse errors, the input string is returned as-is.
func URIToPath(uri DocumentURI) string {
	u, err := url.Parse(string(uri))
	if err != nil {
		s := string(uri)
		if strings.HasPrefix(s, "file://") {
			return strings.TrimPrefix(s, "file://")
		}
		return s
	}
	if u.Scheme == "file" {
		return filepath.Clean(filepath.FromSlash(u.Path))
	}
	return string(uri)
}
