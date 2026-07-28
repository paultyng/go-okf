package okf

import (
	"strings"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
)

// AttestedComputationType is the concept `type` value with normative
// semantics under OKF v0.2 §10: a sanctioned way to compute a value, so a
// consumer can confirm the value was produced by running it. go-okf
// self-registers this single type; every other type string is free-form.
const AttestedComputationType = "Attested Computation"

// Parameter is a typed, named hole an Attested Computation's computation
// may bind (OKF v0.2 §10.2).
type Parameter struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
}

// Executor names how a computation is run and what a run must return.
type Executor struct {
	Resource string   `yaml:"resource,omitempty"`
	Receipt  []string `yaml:"receipt,omitempty"`
}

// Attester names the deterministic (no-LLM) check that inspects a receipt
// and returns a verdict.
type Attester struct {
	Resource string `yaml:"resource,omitempty"`
}

// AttestedComputation is the typed view of a `type: Attested Computation`
// concept (OKF v0.2 §10). It is parse-only: go-okf never runs an executor
// or attester, only records the contract and the means to check it.
//
// Computation holds the `computation:` frontmatter path when present; when
// that key is absent, it instead holds the literal contents of the body's
// `# Computation` fenced or indented code block (§10.3), so callers always
// find "the computation" in one field regardless of which form the
// producer used.
type AttestedComputation struct {
	Runtime     string      `yaml:"runtime"`
	Parameters  []Parameter `yaml:"parameters,omitempty"`
	Computation string      `yaml:"computation,omitempty"`
	Executor    Executor    `yaml:"executor,omitempty"`
	Attester    Attester    `yaml:"attester,omitempty"`
}

func init() {
	Register(AttestedComputationType, func(c *Concept) (any, error) {
		return decodeAttestedComputation(c)
	})
}

func decodeAttestedComputation(c *Concept) (*AttestedComputation, error) {
	ac, err := As[AttestedComputation](c)
	if err != nil {
		return nil, err
	}
	if ac.Computation == "" {
		ac.Computation = extractComputationFence(c.Body)
	}
	return ac, nil
}

// AsAttestedComputation decodes c as an Attested Computation, returning
// (nil, false) when c is not of that type.
func (c *Concept) AsAttestedComputation() (*AttestedComputation, bool) {
	if c.Type != AttestedComputationType {
		return nil, false
	}
	ac, err := decodeAttestedComputation(c)
	if err != nil {
		return nil, false
	}
	return ac, true
}

// Validate implements [Validator]. `runtime` is the only "REQUIRED for this
// type" clause in the OKF v0.2 spec (§10.2).
func (ac *AttestedComputation) Validate() []Violation {
	if strings.TrimSpace(ac.Runtime) == "" {
		return []Violation{{
			Rule:   "attested-computation-runtime-required",
			Detail: "Attested Computation concept is missing the required `runtime` field",
		}}
	}
	return nil
}

var computationFenceMarkdown = goldmark.New()

// extractComputationFence returns the literal contents of the first fenced
// or indented code block under a `# Computation` heading in body, or "" if
// none is found (OKF v0.2 §4.2, §10.3). Parsing is bounded by
// [MaxParseBytes] (see its doc comment) since body is untrusted producer
// input.
func extractComputationFence(body string) string {
	doc, source := parseBody(computationFenceMarkdown, body)

	var result string
	inSection := false
	sectionLevel := 0

	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		if h, ok := n.(*gast.Heading); ok {
			if inSection {
				if h.Level <= sectionLevel {
					return gast.WalkStop, nil
				}
				return gast.WalkSkipChildren, nil
			}
			if strings.EqualFold(strings.TrimSpace(string(nodeText(h, source))), "Computation") {
				inSection = true
				sectionLevel = h.Level
			}
			return gast.WalkSkipChildren, nil
		}
		if inSection {
			switch cb := n.(type) {
			case *gast.FencedCodeBlock:
				result = string(cb.Lines().Value(source))
				return gast.WalkStop, nil
			case *gast.CodeBlock:
				result = string(cb.Lines().Value(source))
				return gast.WalkStop, nil
			}
		}
		return gast.WalkContinue, nil
	})

	return strings.TrimRight(result, "\n")
}
