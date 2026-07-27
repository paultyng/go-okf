package okf

import (
	"net/url"
	"strings"
)

// CanonicalURL normalizes raw for comparison/identity only — it is never
// persisted or displayed; stored and emitted URLs stay verbatim. The
// normalization is deliberately narrow: lowercase scheme and host, strip a
// default port (:80 for http, :443 for https), drop the fragment, keep the
// query string, and treat an empty path as "/". Unparsable input is
// returned unchanged.
func CanonicalURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Host)
	if h, port, ok := splitHostPort(host); ok {
		if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
			host = h
		}
	}

	path := u.Path
	if path == "" {
		path = "/"
	}

	var b strings.Builder
	if scheme != "" {
		b.WriteString(scheme)
		b.WriteString("://")
	}
	b.WriteString(host)
	b.WriteString(path)
	if u.RawQuery != "" {
		b.WriteByte('?')
		b.WriteString(u.RawQuery)
	}
	return b.String()
}

// splitHostPort splits a lowercase "host:port" string. Unlike net.SplitHostPort
// it does not error on a bare host; ok is false when there is no ":".
func splitHostPort(host string) (h, port string, ok bool) {
	i := strings.LastIndex(host, ":")
	if i < 0 {
		return host, "", false
	}
	return host[:i], host[i+1:], true
}
