package okf

import (
	"reflect"
	"testing"
)

func TestTrustTier(t *testing.T) {
	tests := []struct {
		name     string
		verified []Actor
		want     string
	}{
		{"none", nil, "unverified"},
		{"machine only", []Actor{{By: "process:finance-nightly"}}, "machine-confirmed"},
		{"machine then human", []Actor{{By: "process:finance-nightly"}, {By: "human:ahormati"}}, "human-reviewed"},
		{"human only", []Actor{{By: "human:ahormati"}}, "human-reviewed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Concept{Verified: tt.verified}
			if got := c.TrustTier(); got != tt.want {
				t.Errorf("TrustTier() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTrustTierBareMapping(t *testing.T) {
	src := []byte("---\ntype: X\nverified: { by: human:ahormati, at: 2026-06-25T09:00:00Z }\n---\n\nbody\n")
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(c.Verified) != 1 || c.Verified[0].By != "human:ahormati" {
		t.Fatalf("Verified = %+v, want a one-element list", c.Verified)
	}
	if got := c.TrustTier(); got != "human-reviewed" {
		t.Errorf("TrustTier() = %q, want human-reviewed", got)
	}
}

func TestIsStale(t *testing.T) {
	ref := NewDate(2026, 9, 23)

	staleAfter, _ := ParseDate("2026-09-23")
	c := &Concept{StaleAfter: &staleAfter}
	if !c.IsStale(ref) {
		t.Error("expected stale when today == stale_after")
	}

	notYet, _ := ParseDate("2026-09-24")
	c2 := &Concept{StaleAfter: &notYet}
	if c2.IsStale(ref) {
		t.Error("expected not stale when today < stale_after")
	}

	c3 := &Concept{}
	if c3.IsStale(ref) {
		t.Error("expected not stale when stale_after is absent")
	}
}

func TestParseDateTolerantOfDatetimePrefix(t *testing.T) {
	d, err := ParseDate("2026-09-23T14:00:00Z")
	if err != nil {
		t.Fatalf("ParseDate: %v", err)
	}
	if d.Time.Format("2006-01-02") != "2026-09-23" {
		t.Errorf("ParseDate = %v", d.Time)
	}

	if _, err := ParseDate("not-a-date"); err == nil {
		t.Error("expected an error for an unparseable date")
	}
}

func TestUsageWindowRoundTrip(t *testing.T) {
	src := []byte(`---
type: Attested Computation
sources:
  - id: exec-rev-dash
    resource: dashboards/exec-revenue
    usage_count: 5000
    usage_window: { from: 2026-05-01, to: 2026-05-31 }
usage_window: { from: 2026-06-01, to: 2026-06-30 }
---

body
`)
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if c.UsageWindow == nil {
		t.Fatal("expected top-level UsageWindow to be populated")
	}
	if got := c.UsageWindow.From.Time.Format("2006-01-02"); got != "2026-06-01" {
		t.Errorf("UsageWindow.From = %q", got)
	}
	if got := c.UsageWindow.To.Time.Format("2006-01-02"); got != "2026-06-30" {
		t.Errorf("UsageWindow.To = %q", got)
	}

	if len(c.Sources) != 1 || c.Sources[0].UsageWindow == nil {
		t.Fatalf("expected Sources[0].UsageWindow to be populated, got %+v", c.Sources)
	}
	if got := c.Sources[0].UsageWindow.From.Time.Format("2006-01-02"); got != "2026-05-01" {
		t.Errorf("Sources[0].UsageWindow.From = %q", got)
	}
	if got := c.Sources[0].UsageWindow.To.Time.Format("2006-01-02"); got != "2026-05-31" {
		t.Errorf("Sources[0].UsageWindow.To = %q", got)
	}

	out := Marshal(c)
	reparsed, err := Parse(out)
	if err != nil {
		t.Fatalf("re-Parse: %v", err)
	}
	if !reflect.DeepEqual(reparsed, c) {
		t.Errorf("round-trip mismatch:\n got=%+v\nwant=%+v", reparsed, c)
	}
	if reparsed.UsageWindow.From.Time.Format("2006-01-02") != "2026-06-01" {
		t.Errorf("round-tripped top-level UsageWindow.From = %v", reparsed.UsageWindow.From)
	}
	if reparsed.Sources[0].UsageWindow.From.Time.Format("2006-01-02") != "2026-05-01" {
		t.Errorf("round-tripped Sources[0].UsageWindow.From = %v", reparsed.Sources[0].UsageWindow.From)
	}
}

func TestLifecycleStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   string
	}{
		{"absent defaults to stable", "", StatusStable},
		{"explicit draft passes through", StatusDraft, StatusDraft},
		{"explicit deprecated passes through", StatusDeprecated, StatusDeprecated},
		{"unknown value passes through unrejected", "archived", "archived"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Concept{Status: tt.status}
			if got := c.LifecycleStatus(); got != tt.want {
				t.Errorf("LifecycleStatus() = %q, want %q", got, tt.want)
			}
			// LifecycleStatus must not mutate the stored field.
			if c.Status != tt.status {
				t.Errorf("Status mutated: got %q, want %q", c.Status, tt.status)
			}
		})
	}
}

func TestDateYAMLRoundTrip(t *testing.T) {
	src := []byte("---\ntype: X\nstale_after: 2026-09-23\n---\n\nbody\n")
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.StaleAfter == nil || c.StaleAfter.Time.Format("2006-01-02") != "2026-09-23" {
		t.Fatalf("StaleAfter = %v", c.StaleAfter)
	}
	out := Marshal(c)
	reparsed, err := Parse(out)
	if err != nil {
		t.Fatalf("re-Parse: %v", err)
	}
	if reparsed.StaleAfter.Time.Format("2006-01-02") != "2026-09-23" {
		t.Errorf("round-tripped StaleAfter = %v", reparsed.StaleAfter)
	}
}
