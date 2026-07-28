package okf

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const frontmatterDelim = "---"

// Concept is the concrete carrier for a single OKF concept document: a
// markdown file with YAML frontmatter. It is a concrete struct rather than
// an interface, since a concept is data (frontmatter + verbatim body) and
// round-trip safety demands a concrete carrier. Extensible typing is
// layered on top via [As] / [Register] / [Concept.Typed] (see registry.go).
type Concept struct {
	// Core frontmatter (OKF v0.2 §4.1).
	Type        string // REQUIRED — the type discriminator.
	Title       string
	Description string
	Resource    string // singular canonical URI (concept identity).
	Tags        []string

	// Provenance, trust, and lifecycle families (OKF v0.2 §5). All optional.
	Sources     []Source
	UsageWindow *UsageWindow // §5.1 — shared window for Sources[].UsageCount; a [Source] may override it.
	Generated   *Actor       // §5.2 — last content change; supersedes v0.1 `timestamp`.
	Verified    []Actor
	Status      string
	StaleAfter  *Date

	// Extra holds every other frontmatter key, preserved verbatim so
	// round-tripping never silently drops producer-defined fields. Because
	// it is a plain map[string]any (not an order-preserving structure),
	// keys serialize sorted (yaml.v3's default map behavior); authored key
	// order is not preserved.
	Extra map[string]any

	// Body is the markdown body, verbatim and opaque to this package.
	Body string
}

// frontmatterFields is the internal decode target for known frontmatter
// keys. Using a single yaml.Unmarshal pass into typed fields lets Date,
// Actor, and verifiedList's custom (Un)MarshalYAML run directly against the
// original YAML nodes, rather than round-tripping through an intermediate
// map[string]any (which would lose fidelity for date-only scalars).
type frontmatterFields struct {
	Type        string       `yaml:"type"`
	Title       string       `yaml:"title"`
	Description string       `yaml:"description"`
	Resource    string       `yaml:"resource"`
	Tags        []string     `yaml:"tags"`
	Sources     []Source     `yaml:"sources"`
	UsageWindow *UsageWindow `yaml:"usage_window"`
	Generated   *Actor       `yaml:"generated"`
	Verified    verifiedList `yaml:"verified"`
	Status      string       `yaml:"status"`
	StaleAfter  *Date        `yaml:"stale_after"`

	// Timestamp is the legacy v0.1 field, superseded by Generated.At. It is
	// consumed as a fallback (see provenance.go) and never populated onto
	// Concept directly.
	Timestamp string `yaml:"timestamp"`
}

// knownFrontmatterKeys are the keys consumed into typed Concept fields
// (including the legacy `timestamp` fallback); everything else flows into
// Extra.
var knownFrontmatterKeys = map[string]bool{
	"type":         true,
	"title":        true,
	"description":  true,
	"resource":     true,
	"tags":         true,
	"sources":      true,
	"usage_window": true,
	"generated":    true,
	"verified":     true,
	"status":       true,
	"stale_after":  true,
	"timestamp":    true,
}

// splitFrontmatter splits raw document text into a frontmatter block (the
// text between the delimiting `---` lines, or nil if absent) and the body.
// It mirrors the reference implementation exactly: the frontmatter must
// begin on line 1, a missing closing delimiter is an error, and exactly one
// leading newline is stripped from the body.
func splitFrontmatter(text string) (fm []byte, body string, hasFM bool, err error) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != frontmatterDelim {
		return nil, text, false, nil
	}

	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == frontmatterDelim {
			endIdx = i
			break
		}
	}
	if endIdx == -1 {
		return nil, "", false, fmt.Errorf("okf: unterminated YAML frontmatter block")
	}

	fmText := strings.Join(lines[1:endIdx], "\n")
	b := strings.Join(lines[endIdx+1:], "\n")
	b = strings.TrimPrefix(b, "\n")
	return []byte(fmText), b, true, nil
}

// Parse parses a concept document's raw bytes into a [Concept]. It tolerates
// documents with no frontmatter (frontmatter left zero-valued, all content
// treated as body) but returns an error for an unterminated frontmatter
// block or frontmatter that isn't a YAML mapping.
func Parse(data []byte) (*Concept, error) {
	fmBytes, body, hasFM, err := splitFrontmatter(string(data))
	if err != nil {
		return nil, err
	}

	c := &Concept{Body: body}
	if !hasFM {
		return c, nil
	}

	var known frontmatterFields
	if err := yaml.Unmarshal(fmBytes, &known); err != nil {
		return nil, fmt.Errorf("okf: invalid YAML in frontmatter: %w", err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(fmBytes, &raw); err != nil {
		return nil, fmt.Errorf("okf: invalid YAML in frontmatter: %w", err)
	}

	c.Type = known.Type
	c.Title = known.Title
	c.Description = known.Description
	c.Resource = known.Resource
	c.Tags = known.Tags
	c.Sources = known.Sources
	c.UsageWindow = known.UsageWindow
	c.Generated = known.Generated
	c.Verified = []Actor(known.Verified)
	c.Status = known.Status
	c.StaleAfter = known.StaleAfter

	// Legacy v0.1 fallback: `timestamp` populates Generated.At only when
	// `generated` itself is absent (OKF v0.2 §13.1).
	if c.Generated == nil && known.Timestamp != "" {
		if at, ok := parseLegacyTimestamp(known.Timestamp); ok {
			c.Generated = &Actor{At: at}
		}
	}

	extra := map[string]any{}
	for k, v := range raw {
		if !knownFrontmatterKeys[k] {
			extra[k] = v
		}
	}
	if len(extra) > 0 {
		c.Extra = extra
	}

	return c, nil
}

// frontmatterMap reconstructs the full frontmatter as a map[string]any for
// marshaling: Extra keys plus every non-zero-valued core field. Optional
// fields are omitted entirely when zero-valued, so a minimal concept
// (`type` only) round-trips to a minimal frontmatter block.
func (c *Concept) frontmatterMap() map[string]any {
	m := make(map[string]any, len(c.Extra)+8)
	for k, v := range c.Extra {
		m[k] = v
	}
	m["type"] = c.Type
	if c.Title != "" {
		m["title"] = c.Title
	}
	if c.Description != "" {
		m["description"] = c.Description
	}
	if c.Resource != "" {
		m["resource"] = c.Resource
	}
	if len(c.Tags) > 0 {
		m["tags"] = c.Tags
	}
	if len(c.Sources) > 0 {
		m["sources"] = c.Sources
	}
	if c.UsageWindow != nil {
		m["usage_window"] = c.UsageWindow
	}
	if c.Generated != nil {
		m["generated"] = c.Generated
	}
	if len(c.Verified) > 0 {
		m["verified"] = c.Verified
	}
	if c.Status != "" {
		m["status"] = c.Status
	}
	if c.StaleAfter != nil {
		m["stale_after"] = c.StaleAfter
	}
	return m
}

// Marshal serializes a [Concept] back into a full document: frontmatter
// delimited by `---` lines followed by the verbatim body. Marshal is pure
// (no I/O); callers persist the returned bytes (see the filesystem
// abstraction notes in the package doc).
func Marshal(c *Concept) []byte {
	fmBytes, err := yaml.Marshal(c.frontmatterMap())
	if err != nil {
		// yaml.Marshal over these known-safe types practically never
		// fails; fall back to a minimal, always-valid document rather than
		// panicking or silently dropping data.
		fmBytes = []byte("type: " + c.Type)
	}
	fmText := strings.TrimRight(string(fmBytes), "\n")

	body := c.Body
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}

	var buf bytes.Buffer
	buf.WriteString(frontmatterDelim)
	buf.WriteByte('\n')
	buf.WriteString(fmText)
	buf.WriteByte('\n')
	buf.WriteString(frontmatterDelim)
	buf.WriteString("\n\n")
	buf.WriteString(body)
	return buf.Bytes()
}
