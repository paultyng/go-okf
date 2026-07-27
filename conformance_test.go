package okf

import "testing"

func TestConformanceEmptyBundleIsConformant(t *testing.T) {
	b := FromConcepts(map[string]*Concept{})
	if got := b.Conformance(); len(got) != 0 {
		t.Errorf("Conformance() = %+v, want none", got)
	}
}

func TestConformanceFlagsMissingType(t *testing.T) {
	b := FromConcepts(map[string]*Concept{
		"a": {Type: "X"},
		"b": {Type: ""},
	})
	violations := b.Conformance()
	if len(violations) != 1 {
		t.Fatalf("Conformance() = %+v, want exactly one violation", violations)
	}
	if violations[0].ConceptID != "b" || violations[0].Rule != "type-required" {
		t.Errorf("violation = %+v", violations[0])
	}
}

func TestConformancePermissiveAboutUnknownTypeAndExtraKeys(t *testing.T) {
	b := FromConcepts(map[string]*Concept{
		"a": {Type: "SomeNeverBeforeSeenType", Extra: map[string]any{"whatever": 1}},
	})
	if got := b.Conformance(); len(got) != 0 {
		t.Errorf("Conformance() = %+v, want none (unknown type/keys are not violations)", got)
	}
}

func TestConformanceTypeOnlyIsFullyConformant(t *testing.T) {
	// OKF v0.2 §11: `type` is the only always-required frontmatter key.
	c, err := Parse([]byte("---\ntype: X\n---\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	b := FromConcepts(map[string]*Concept{"a": c})
	if got := b.Conformance(); len(got) != 0 {
		t.Errorf("Conformance() = %+v, want none", got)
	}
}
