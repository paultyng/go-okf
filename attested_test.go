package okf

import "testing"

const attestedComputationDoc = `---
type: Attested Computation
title: Revenue for fiscal year
runtime: bigquery
parameters:
  - { name: year, type: integer, required: true }
executor:
  resource: references/skills/run-on-bq.md
  receipt: [job_id, executed_sql, result]
attester:
  resource: references/attesters/revenue.py
---

# Computation

    SELECT SUM(amount) AS revenue
    FROM finance.recognized_revenue
    WHERE fiscal_year = @year

Recognized revenue per the recognition policy.
`

func TestAttestedComputation(t *testing.T) {
	c, err := Parse([]byte(attestedComputationDoc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	ac, ok := c.AsAttestedComputation()
	if !ok {
		t.Fatal("expected AsAttestedComputation to succeed")
	}
	if ac.Runtime != "bigquery" {
		t.Errorf("Runtime = %q", ac.Runtime)
	}
	if len(ac.Parameters) != 1 || ac.Parameters[0].Name != "year" || !ac.Parameters[0].Required {
		t.Errorf("Parameters = %+v", ac.Parameters)
	}
	if ac.Executor.Resource != "references/skills/run-on-bq.md" {
		t.Errorf("Executor.Resource = %q", ac.Executor.Resource)
	}
	if len(ac.Executor.Receipt) != 3 {
		t.Errorf("Executor.Receipt = %v", ac.Executor.Receipt)
	}

	wantFence := "SELECT SUM(amount) AS revenue\nFROM finance.recognized_revenue\nWHERE fiscal_year = @year"
	if ac.Computation != wantFence {
		t.Errorf("Computation (fence-extracted) = %q, want %q", ac.Computation, wantFence)
	}
}

func TestAttestedComputationExplicitPathSkipsFenceExtraction(t *testing.T) {
	src := []byte(`---
type: Attested Computation
runtime: dbt
computation: references/computations/lib/revenue.sql
---

# Computation

    this text should be ignored
`)
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ac, ok := c.AsAttestedComputation()
	if !ok {
		t.Fatal("expected AsAttestedComputation to succeed")
	}
	if ac.Computation != "references/computations/lib/revenue.sql" {
		t.Errorf("Computation = %q, want the frontmatter path preserved", ac.Computation)
	}
}

func TestAttestedComputationNotThatType(t *testing.T) {
	c := &Concept{Type: "Metric"}
	if _, ok := c.AsAttestedComputation(); ok {
		t.Error("expected AsAttestedComputation to fail for a non-matching type")
	}
}

func TestAttestedComputationValidateRequiresRuntime(t *testing.T) {
	ac := &AttestedComputation{}
	violations := ac.Validate()
	if len(violations) != 1 || violations[0].Rule != "attested-computation-runtime-required" {
		t.Errorf("Validate() = %+v, want one runtime-required violation", violations)
	}

	ac2 := &AttestedComputation{Runtime: "postgres"}
	if v := ac2.Validate(); len(v) != 0 {
		t.Errorf("Validate() = %+v, want no violations when runtime is set", v)
	}
}

func TestAttestedComputationSelfRegistersAndFeedsConformance(t *testing.T) {
	c, err := Parse([]byte("---\ntype: Attested Computation\n---\n\nbody\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	b := FromConcepts(map[string]*Concept{"computations/missing-runtime": c})
	violations := b.Conformance()
	if len(violations) != 1 || violations[0].Rule != "attested-computation-runtime-required" {
		t.Fatalf("Conformance() = %+v", violations)
	}
	if violations[0].ConceptID != "computations/missing-runtime" {
		t.Errorf("ConceptID = %q", violations[0].ConceptID)
	}
}
