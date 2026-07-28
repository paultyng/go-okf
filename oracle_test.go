// Table tests ported from the GCP Python reference implementation
// (reference_agent/bundle/{document,index}.py and their pytest suites,
// test_document.py + test_index.py) for behavioral parity. Each test names
// its Python origin. Concept is a typed struct rather than a frontmatter
// dict, and go-okf's permissive-by-default Parse defers the "type is
// required" check to Bundle.Conformance (OKF v0.2 §11 is a bundle-level
// conformance concept, not a per-document exception) — those two tests are
// translated accordingly, noted inline.
package okf

import (
	"reflect"
	"strings"
	"testing"
)

// Ported from test_document.py::test_roundtrip_preserves_frontmatter_and_body.
func TestOracleRoundtripPreservesFrontmatterAndBody(t *testing.T) {
	src := []byte("---\n" +
		"type: BigQuery Table\n" +
		"title: Sample\n" +
		"description: A sample table.\n" +
		"tags: [a, b]\n" +
		"---\n" +
		"\n" +
		"# Sample\n" +
		"\n" +
		"Body text.\n")

	c, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.Type != "BigQuery Table" {
		t.Errorf("Type = %q", c.Type)
	}
	if !reflect.DeepEqual(c.Tags, []string{"a", "b"}) {
		t.Errorf("Tags = %v", c.Tags)
	}
	if !strings.HasPrefix(c.Body, "# Sample") {
		t.Errorf("Body = %q, want it to start with \"# Sample\"", c.Body)
	}

	reparsed, err := Parse(c.Bytes())
	if err != nil {
		t.Fatalf("re-Parse: %v", err)
	}
	if !reflect.DeepEqual(reparsed, c) {
		t.Errorf("frontmatter mismatch after round-trip:\n got=%+v\nwant=%+v", reparsed, c)
	}
	if strings.TrimRight(reparsed.Body, "\n") != strings.TrimRight(c.Body, "\n") {
		t.Errorf("body mismatch after round-trip: %q vs %q", reparsed.Body, c.Body)
	}
}

// Ported from test_document.py::test_parse_no_frontmatter_treats_all_as_body.
func TestOracleParseNoFrontmatterTreatsAllAsBody(t *testing.T) {
	src := []byte("# Hello\n\nNo frontmatter here.\n")
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.Type != "" {
		t.Errorf("Type = %q, want empty", c.Type)
	}
	if !strings.Contains(c.Body, "Hello") {
		t.Errorf("Body = %q, want it to contain %q", c.Body, "Hello")
	}
}

// Ported from test_document.py::test_unterminated_frontmatter_raises.
func TestOracleUnterminatedFrontmatterRaises(t *testing.T) {
	src := []byte("---\ntype: X\nstill in frontmatter\n")
	if _, err := Parse(src); err == nil {
		t.Fatal("expected Parse to error on an unterminated frontmatter block")
	}
}

// Ported from test_document.py::test_validate_rejects_missing_type.
//
// The Python OKFDocument.validate() raises per-document; go-okf's
// permissive Parse never errors on a missing `type` (OKF v0.2 §11 makes
// that a bundle Conformance concern), so this is translated to a
// Conformance check instead.
func TestOracleValidateRejectsMissingType(t *testing.T) {
	b := FromConcepts(map[string]*Concept{"y": {Title: "Y"}})
	violations := b.Conformance()
	if len(violations) != 1 || violations[0].Rule != "type-required" {
		t.Errorf("Conformance() = %+v, want one type-required violation", violations)
	}
}

// Ported from test_document.py::test_validate_accepts_type_only.
func TestOracleValidateAcceptsTypeOnly(t *testing.T) {
	b := FromConcepts(map[string]*Concept{"x": {Type: "X"}})
	if got := b.Conformance(); len(got) != 0 {
		t.Errorf("Conformance() = %+v, want none for a type-only concept", got)
	}
}

// Ported from test_document.py::test_normalize_verified_treats_bare_mapping_as_list.
func TestOracleNormalizeVerifiedTreatsBareMappingAsList(t *testing.T) {
	src := []byte("---\ntype: X\nverified: { by: human:ahormati, at: 2026-06-25T09:00:00Z }\n---\n\nbody\n")
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(c.Verified) != 1 || c.Verified[0].By != "human:ahormati" {
		t.Errorf("Verified = %+v, want a one-element list", c.Verified)
	}

	empty := &Concept{}
	if len(empty.Verified) != 0 {
		t.Errorf("Verified = %+v, want none", empty.Verified)
	}
}

// Ported from test_document.py::test_trust_tier.
func TestOracleTrustTier(t *testing.T) {
	if got := (&Concept{}).TrustTier(); got != "unverified" {
		t.Errorf("TrustTier() = %q, want unverified", got)
	}
	if got := (&Concept{Verified: []Actor{{By: "process:finance-nightly"}}}).TrustTier(); got != "machine-confirmed" {
		t.Errorf("TrustTier() = %q, want machine-confirmed", got)
	}
	if got := (&Concept{Verified: []Actor{
		{By: "process:finance-nightly"},
		{By: "human:ahormati"},
	}}).TrustTier(); got != "human-reviewed" {
		t.Errorf("TrustTier() = %q, want human-reviewed", got)
	}

	src := []byte("---\ntype: X\nverified: { by: human:ahormati, at: 2026-06-25T09:00:00Z }\n---\n\nbody\n")
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := c.TrustTier(); got != "human-reviewed" {
		t.Errorf("TrustTier() = %q, want human-reviewed for a bare verified mapping", got)
	}
}

// Ported from test_document.py::test_is_stale.
func TestOracleIsStale(t *testing.T) {
	ref := NewDate(2026, 9, 23)

	d1, _ := ParseDate("2026-09-23")
	if !(&Concept{StaleAfter: &d1}).IsStale(ref) {
		t.Error("expected stale when today == stale_after")
	}

	d2, _ := ParseDate("2026-09-24")
	if (&Concept{StaleAfter: &d2}).IsStale(ref) {
		t.Error("expected not stale when today < stale_after")
	}

	if (&Concept{}).IsStale(ref) {
		t.Error("expected not stale when stale_after is absent")
	}

	// "not-a-date" is unparseable: per OKF v0.2 §11 permissive parsing (and
	// matching the Python reference's silent is_stale(..., today=ref) ->
	// False), Parse must not fail on a bad optional date. Date.UnmarshalYAML
	// stores the zero Date instead, which IsStale treats as not stale.
	bad, err := Parse([]byte("---\ntype: X\nstale_after: not-a-date\n---\n\nbody\n"))
	if err != nil {
		t.Fatalf("Parse: %v, want no error for an unparseable stale_after", err)
	}
	if bad.IsStale(ref) {
		t.Error("expected not stale when stale_after was unparseable")
	}
}
