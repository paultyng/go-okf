package okf

import "testing"

func TestCanonicalURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"lowercases scheme and host", "HTTPS://Example.COM/path", "https://example.com/path"},
		{"strips default https port", "https://example.com:443/path", "https://example.com/path"},
		{"strips default http port", "http://example.com:80/path", "http://example.com/path"},
		{"keeps non-default port", "https://example.com:8443/path", "https://example.com:8443/path"},
		{"drops fragment", "https://example.com/path#section", "https://example.com/path"},
		{"keeps query", "https://example.com/path?a=1&b=2", "https://example.com/path?a=1&b=2"},
		{"empty path becomes slash", "https://example.com", "https://example.com/"},
		{"bundle-relative path unchanged", "/tables/customers.md", "/tables/customers.md"},
		{"equivalent urls canonicalize equal", "HTTPS://Example.com:443/x#y", "https://example.com/x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanonicalURL(tt.in); got != tt.want {
				t.Errorf("CanonicalURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCanonicalURLNeverMutatesStoredForm(t *testing.T) {
	// CanonicalURL is a pure comparison function; verify two differently
	// authored but equivalent URLs canonicalize to the same key without
	// either original string being altered in place.
	a := "https://Example.com:443/x?q=1"
	b := "HTTPS://example.com/x?q=1"
	if CanonicalURL(a) != CanonicalURL(b) {
		t.Errorf("expected %q and %q to canonicalize equal", a, b)
	}
	if a != "https://Example.com:443/x?q=1" {
		t.Error("input string a was mutated")
	}
}
