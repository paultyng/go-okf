package okf

import "strings"

// Violation is one conformance finding (OKF v0.2 §11). ConceptID is filled
// in by [Bundle.Conformance] when a [Validator] implementation leaves it
// empty, so individual Validate() implementations don't need bundle
// context.
type Violation struct {
	ConceptID string
	Rule      string
	Detail    string
}

// Conformance checks every concept in the bundle against OKF v0.2 §11: the
// base rule (every concept has a non-empty `type`) plus any rules
// contributed by a registered type's [Validator] implementation. An empty
// return slice means the bundle is conformant. Conformance is permissive
// per spec: unknown types, unknown keys, broken links, and missing
// optional fields are never violations.
func (b *Bundle) Conformance() []Violation {
	var out []Violation
	for _, id := range b.sortedIDs() {
		c := b.concepts[ConceptID(id)]

		if strings.TrimSpace(c.Type) == "" {
			out = append(out, Violation{
				ConceptID: id,
				Rule:      "type-required",
				Detail:    "concept has no `type`",
			})
			continue
		}

		v, registered, err := decodeRegistered(c)
		if !registered {
			continue
		}
		if err != nil {
			out = append(out, Violation{
				ConceptID: id,
				Rule:      "decode",
				Detail:    err.Error(),
			})
			continue
		}
		validator, ok := v.(Validator)
		if !ok {
			continue
		}
		for _, violation := range validator.Validate() {
			if violation.ConceptID == "" {
				violation.ConceptID = id
			}
			out = append(out, violation)
		}
	}
	if out == nil {
		return []Violation{}
	}
	return out
}
