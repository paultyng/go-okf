package okf

import (
	"reflect"
	"testing"
)

func TestConceptRoundTrip(t *testing.T) {
	src := []byte(`---
type: BigQuery Table
title: Sample
description: A sample table.
tags: [a, b]
generated: { by: reference_agent/gemini-2.5-pro, at: 2026-05-28T00:00:00Z }
custom_key: custom_value
---

# Sample

Body text.
`)
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
	if c.Extra["custom_key"] != "custom_value" {
		t.Errorf("Extra[custom_key] = %v", c.Extra["custom_key"])
	}
	if c.Body != "# Sample\n\nBody text.\n" {
		t.Errorf("Body = %q", c.Body)
	}

	out := Marshal(c)
	reparsed, err := Parse(out)
	if err != nil {
		t.Fatalf("re-Parse: %v", err)
	}
	if !reflect.DeepEqual(reparsed, c) {
		t.Errorf("round-trip mismatch:\n got=%+v\nwant=%+v", reparsed, c)
	}
}

func TestConceptRoundTripMinimal(t *testing.T) {
	src := []byte("---\ntype: X\n---\n\nbody\n")
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out := Marshal(c)
	reparsed, err := Parse(out)
	if err != nil {
		t.Fatalf("re-Parse: %v", err)
	}
	if !reflect.DeepEqual(reparsed, c) {
		t.Errorf("round-trip mismatch:\n got=%+v\nwant=%+v", reparsed, c)
	}
}

func TestParseNoFrontmatterTreatsAllAsBody(t *testing.T) {
	src := []byte("# Hello\n\nNo frontmatter here.\n")
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.Type != "" || c.Extra != nil {
		t.Errorf("expected zero-value frontmatter, got Type=%q Extra=%v", c.Type, c.Extra)
	}
	if c.Body != string(src) {
		t.Errorf("Body = %q, want verbatim source", c.Body)
	}
}

func TestParseUnterminatedFrontmatterErrors(t *testing.T) {
	src := []byte("---\ntype: X\nstill in frontmatter\n")
	if _, err := Parse(src); err == nil {
		t.Fatal("expected an error for unterminated frontmatter")
	}
}

func TestParseLegacyTimestampFallback(t *testing.T) {
	src := []byte("---\ntype: Metric\ntimestamp: '2026-05-28T22:53:05+00:00'\n---\n\nbody\n")
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.Generated == nil {
		t.Fatal("expected Generated to be populated from legacy timestamp")
	}
	want := "2026-05-28T22:53:05Z"
	if got := c.Generated.At.UTC().Format("2006-01-02T15:04:05Z"); got != want {
		t.Errorf("Generated.At = %q, want %q", got, want)
	}
}

func TestParseGeneratedTakesPrecedenceOverTimestamp(t *testing.T) {
	src := []byte(`---
type: Metric
timestamp: '2020-01-01T00:00:00+00:00'
generated: { by: human:x, at: 2026-06-20T22:53:05Z }
---

body
`)
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.Generated.By != "human:x" {
		t.Errorf("Generated.By = %q, want human:x (generated should win over legacy timestamp)", c.Generated.By)
	}
}
