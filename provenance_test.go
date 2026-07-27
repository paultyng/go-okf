package okf

import "testing"

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
