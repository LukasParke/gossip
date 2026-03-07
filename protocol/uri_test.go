package protocol

import (
	"testing"
)

func TestNormalizeURI(t *testing.T) {
	tests := []struct {
		name     string
		input    DocumentURI
		expected DocumentURI
	}{
		{
			name:     "clean file URI unchanged",
			input:    "file:///home/user/test.yaml",
			expected: "file:///home/user/test.yaml",
		},
		{
			name:     "trailing slash removed",
			input:    "file:///home/user/test.yaml/",
			expected: "file:///home/user/test.yaml",
		},
		{
			name:     "dot segments resolved",
			input:    "file:///home/user/sub/../test.yaml",
			expected: "file:///home/user/test.yaml",
		},
		{
			name:     "double dot segments resolved",
			input:    "file:///home/user/a/b/../../test.yaml",
			expected: "file:///home/user/test.yaml",
		},
		{
			name:     "current dir segment removed",
			input:    "file:///home/user/./test.yaml",
			expected: "file:///home/user/test.yaml",
		},
		{
			name:     "fragment stripped",
			input:    "file:///home/user/test.yaml#/components/schemas/Pet",
			expected: "file:///home/user/test.yaml",
		},
		{
			name:     "query stripped",
			input:    "file:///home/user/test.yaml?version=3",
			expected: "file:///home/user/test.yaml",
		},
		{
			name:     "host stripped",
			input:    "file://localhost/home/user/test.yaml",
			expected: "file:///home/user/test.yaml",
		},
		{
			name:     "empty URI unchanged",
			input:    "",
			expected: "",
		},
		{
			name:     "non-file URI unchanged",
			input:    "https://example.com/api.yaml",
			expected: "https://example.com/api.yaml",
		},
		{
			name:     "untitled URI unchanged",
			input:    "untitled:Untitled-1",
			expected: "untitled:Untitled-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeURI(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeURI(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeURI_Idempotent(t *testing.T) {
	uris := []DocumentURI{
		"file:///home/user/test.yaml",
		"file:///home/user/sub/../test.yaml",
		"file:///home/user/./test.yaml",
		"file://localhost/home/user/test.yaml",
	}
	for _, uri := range uris {
		first := NormalizeURI(uri)
		second := NormalizeURI(first)
		if first != second {
			t.Errorf("NormalizeURI not idempotent: %q → %q → %q", uri, first, second)
		}
	}
}

func TestNormalizeURI_ConsistentKeys(t *testing.T) {
	uris := []DocumentURI{
		"file:///home/user/test.yaml",
		"file:///home/user/./test.yaml",
		"file:///home/user/sub/../test.yaml",
		"file://localhost/home/user/test.yaml",
	}
	first := NormalizeURI(uris[0])
	for _, uri := range uris[1:] {
		got := NormalizeURI(uri)
		if got != first {
			t.Errorf("NormalizeURI(%q) = %q, want %q (same as base)", uri, got, first)
		}
	}
}

func TestURIToPath(t *testing.T) {
	tests := []struct {
		name     string
		input    DocumentURI
		expected string
	}{
		{
			name:     "file URI to path",
			input:    "file:///home/user/test.yaml",
			expected: "/home/user/test.yaml",
		},
		{
			name:     "file URI with dot segments cleaned",
			input:    "file:///home/user/sub/../test.yaml",
			expected: "/home/user/test.yaml",
		},
		{
			name:     "non-file URI returned as-is",
			input:    "https://example.com/api.yaml",
			expected: "https://example.com/api.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := URIToPath(tt.input)
			if got != tt.expected {
				t.Errorf("URIToPath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
