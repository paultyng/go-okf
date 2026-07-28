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

func TestConformanceSurfacesDecodeFailureForRegisteredType(t *testing.T) {
	// AttestedComputationType is registered by attested.go's init(); its
	// Parameters field is []Parameter, so a scalar `parameters` value fails
	// to decode.
	malformed := &Concept{
		Type:  AttestedComputationType,
		Extra: map[string]any{"parameters": "not-a-list"},
	}
	violations := FromConcepts(map[string]*Concept{"a": malformed}).Conformance()
	if len(violations) != 1 {
		t.Fatalf("Conformance() = %+v, want exactly one violation for a malformed registered concept", violations)
	}
	if violations[0].ConceptID != "a" || violations[0].Rule != "decode" {
		t.Errorf("violation = %+v", violations[0])
	}

	wellFormed := &Concept{
		Type:  AttestedComputationType,
		Extra: map[string]any{"runtime": "python3.11"},
	}
	if got := FromConcepts(map[string]*Concept{"a": wellFormed}).Conformance(); len(got) != 0 {
		t.Errorf("Conformance() = %+v, want none for a well-formed Attested Computation", got)
	}

	unregistered := &Concept{Type: "SomeUnregisteredType", Extra: map[string]any{"parameters": "not-a-list"}}
	if got := FromConcepts(map[string]*Concept{"a": unregistered}).Conformance(); len(got) != 0 {
		t.Errorf("Conformance() = %+v, want none for an unregistered type (permissive)", got)
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
