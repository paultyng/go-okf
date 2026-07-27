package okf

import (
	"fmt"
	"regexp"
	"strings"
)

// segmentRe matches a single valid concept-id path segment.
var segmentRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.\-]*$`)

// ConceptID is a concept's path within a bundle, with the `.md` suffix
// removed (OKF v0.2 §2), stored as its `/`-joined segments.
type ConceptID string

// ParseConceptID strictly validates s as a concept id: every `/`-separated
// segment must match [A-Za-z0-9_][A-Za-z0-9_.\-]*. Use this to validate an
// id supplied by a caller (e.g. a link target); use ConceptIDFromPath for
// permissively deriving an id from whatever is actually on disk.
func ParseConceptID(s string) (ConceptID, error) {
	parts := nonEmptySegments(s)
	if len(parts) == 0 {
		return "", fmt.Errorf("okf: empty concept id %q", s)
	}
	for _, p := range parts {
		if !segmentRe.MatchString(p) {
			return "", fmt.Errorf("okf: invalid concept id segment %q in %q", p, s)
		}
	}
	return ConceptID(strings.Join(parts, "/")), nil
}

// ConceptIDFromPath permissively derives a concept id from a bundle-relative
// file path (e.g. "tables/events_.md" -> "tables/events_"), accepting
// whatever is on disk without segment validation.
func ConceptIDFromPath(path string) ConceptID {
	path = strings.TrimSuffix(path, ".md")
	parts := nonEmptySegments(path)
	return ConceptID(strings.Join(parts, "/"))
}

func nonEmptySegments(s string) []string {
	raw := strings.Split(s, "/")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
