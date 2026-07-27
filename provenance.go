package okf

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Source records a material a concept derives from (OKF v0.2 §5.1).
type Source struct {
	ID           string       `yaml:"id,omitempty"`
	Resource     string       `yaml:"resource"`
	Title        string       `yaml:"title,omitempty"`
	Author       string       `yaml:"author,omitempty"`
	UsageCount   int          `yaml:"usage_count,omitempty"`
	LastModified *Date        `yaml:"last_modified,omitempty"`
	UsageWindow  *UsageWindow `yaml:"usage_window,omitempty"`
}

// UsageWindow frames a source's usage_count with a date range (OKF v0.2
// §5.1). Written once as a sibling of `sources` (Concept.UsageWindow); a
// single Source entry MAY carry its own UsageWindow to override the shared
// one.
type UsageWindow struct {
	From *Date `yaml:"from,omitempty"`
	To   *Date `yaml:"to,omitempty"`
}

// Actor identifies who or what performed an action (OKF v0.2 §7): an agent
// as "<producer>/<version>", a person as "human:<id>", or an automated
// process as "process:<id>". At is a full ISO 8601 datetime, distinct from
// Date (a bare calendar date).
type Actor struct {
	By string    `yaml:"by"`
	At time.Time `yaml:"at"`
}

// verifiedList decodes the `verified` frontmatter key, which may be either
// a single bare `{ by, at }` mapping or a YAML sequence of such mappings
// (OKF v0.2 §5.2). Consumers MUST treat a bare mapping as a one-element
// list; this type does that normalization at decode time.
type verifiedList []Actor

func (v *verifiedList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case 0:
		*v = nil
	case yaml.MappingNode:
		var a Actor
		if err := node.Decode(&a); err != nil {
			return err
		}
		*v = verifiedList{a}
	case yaml.SequenceNode:
		var as []Actor
		if err := node.Decode(&as); err != nil {
			return err
		}
		*v = verifiedList(as)
	default:
		return fmt.Errorf("okf: `verified` must be a mapping or a sequence of mappings")
	}
	return nil
}

// Date is an ISO 8601 full-date (YYYY-MM-DD), used by `stale_after` (§5.5),
// `sources[].last_modified` (§5.1), and log headings (§9) — distinct from
// Actor.At, a full datetime. Parsing tolerates a leading date within a
// longer datetime string (the first 10 characters), matching the reference
// implementation's is_stale behavior.
type Date struct {
	time.Time
}

// NewDate constructs a Date from year/month/day, matching time.Date's
// component semantics (UTC, midnight).
func NewDate(year int, month time.Month, day int) Date {
	return Date{time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

// ParseDate parses a string as an ISO 8601 full-date, tolerating a leading
// date within a longer datetime string (first 10 characters).
func ParseDate(s string) (Date, error) {
	if len(s) < 10 {
		return Date{}, fmt.Errorf("okf: invalid date %q", s)
	}
	t, err := time.Parse("2006-01-02", s[:10])
	if err != nil {
		return Date{}, fmt.Errorf("okf: invalid date %q: %w", s, err)
	}
	return Date{t}, nil
}

// MarshalYAML implements yaml.Marshaler, emitting YYYY-MM-DD.
func (d Date) MarshalYAML() (any, error) {
	return d.Time.Format("2006-01-02"), nil
}

// UnmarshalYAML implements yaml.Unmarshaler. It reads the node's literal
// scalar value directly (rather than node.Decode into a typed value) so
// that both plain string dates and YAML's implicitly-resolved !!timestamp
// scalars parse identically.
//
// A syntactically-valid document with a value that isn't a parseable date
// (wrong kind, or a scalar like "not-a-date") must never fail Concept
// parsing — permissive parsing (OKF v0.2 §11) treats a bad optional date
// as absent rather than an error. Callers distinguish "absent" via
// Date.Time.IsZero() (see IsStale).
func (d *Date) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		*d = Date{}
		return nil
	}
	parsed, err := ParseDate(node.Value)
	if err != nil {
		*d = Date{}
		return nil
	}
	*d = parsed
	return nil
}

// parseLegacyTimestamp parses the legacy v0.1 `timestamp` field (a full
// ISO 8601 datetime) for the Generated.At fallback (§13.1). It tries a
// full datetime first and falls back to a bare date.
func parseLegacyTimestamp(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// TrustTier derives a concept's trust tier from Verified (OKF v0.2 §5.3):
//   - no verification events => "unverified"
//   - verified only by non-`human:` actors => "machine-confirmed"
//   - verified by any `human:<id>` actor => "human-reviewed"
func (c *Concept) TrustTier() string {
	if len(c.Verified) == 0 {
		return "unverified"
	}
	for _, v := range c.Verified {
		if strings.HasPrefix(v.By, "human:") {
			return "human-reviewed"
		}
	}
	return "machine-confirmed"
}

// Lifecycle status values (OKF v0.2 §5.4). Status is a free string on
// Concept — these constants name the spec's fixed vocabulary without
// restricting parsing to it (permissive parsing never rejects an unknown
// value).
const (
	StatusDraft      = "draft"
	StatusStable     = "stable"
	StatusDeprecated = "deprecated"
)

// LifecycleStatus returns the concept's lifecycle status (OKF v0.2 §5.4),
// defaulting to StatusStable when Status is absent. An unknown Status value
// passes through unchanged rather than being rejected or normalized.
func (c *Concept) LifecycleStatus() string {
	if c.Status == "" {
		return StatusStable
	}
	return c.Status
}

// IsStale reports whether the concept is stale per `stale_after` (OKF v0.2
// §5.5): today >= StaleAfter. today is supplied by the caller — this
// package never calls time.Now. Returns false when StaleAfter is absent or
// was unparseable at parse time (a zero Date), matching the reference
// implementation's is_stale.
func (c *Concept) IsStale(today Date) bool {
	if c.StaleAfter == nil || c.StaleAfter.Time.IsZero() {
		return false
	}
	return !today.Time.Before(c.StaleAfter.Time)
}
