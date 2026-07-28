package okf_test

import (
	"fmt"
	"testing/fstest"
	"time"

	okf "github.com/paultyng/go-okf"
)

// ExampleParse parses a concept bound to a resource (OKF v0.2 §4.3) and
// reads its core frontmatter fields.
func ExampleParse() {
	doc := []byte(`---
type: BigQuery Table
title: Customer Orders
description: One row per completed customer order across all channels.
resource: https://console.cloud.google.com/bigquery?p=acme&d=sales&t=orders
tags: [sales, orders, revenue]
---

# Schema

One row per completed order.
`)

	c, err := okf.Parse(doc)
	if err != nil {
		panic(err)
	}

	fmt.Println(c.Type)
	fmt.Println(c.Title)
	fmt.Println(c.Resource)
	fmt.Println(c.Tags)
	// Output:
	// BigQuery Table
	// Customer Orders
	// https://console.cloud.google.com/bigquery?p=acme&d=sales&t=orders
	// [sales orders revenue]
}

// ExampleConcept_Bytes round-trips a concept through Parse and Bytes, showing
// that an unrecognized frontmatter key (Extra) and the body survive intact.
func ExampleConcept_Bytes() {
	doc := []byte(`---
type: Playbook
title: Incident response
owner: team:sre
---

# Steps

1. Check the dashboard.
`)

	c, err := okf.Parse(doc)
	if err != nil {
		panic(err)
	}
	fmt.Println(c.Extra["owner"])

	out := c.Bytes()
	reparsed, err := okf.Parse(out)
	if err != nil {
		panic(err)
	}
	fmt.Println(reparsed.Extra["owner"])
	fmt.Println(reparsed.Body == c.Body)
	// Output:
	// team:sre
	// team:sre
	// true
}

// ExampleLoad builds a Bundle from an in-memory filesystem and lists its
// concepts in concept-id order.
func ExampleLoad() {
	fsys := fstest.MapFS{
		"tables/customers.md": &fstest.MapFile{Data: []byte("---\ntype: BigQuery Table\ntitle: Customers\n---\n\nbody\n")},
		"tables/orders.md":    &fstest.MapFile{Data: []byte("---\ntype: BigQuery Table\ntitle: Orders\n---\n\nbody\n")},
	}

	b, err := okf.Load(fsys)
	if err != nil {
		panic(err)
	}

	for _, c := range b.Concepts() {
		fmt.Println(c.Title)
	}
	// Output:
	// Customers
	// Orders
}

// ExampleBundle_ExternalLinks merges a concept's resource, sources, and
// inline body links into one deduplicated, origin-tagged list.
func ExampleBundle_ExternalLinks() {
	c, err := okf.Parse([]byte(`---
type: BigQuery Table
resource: https://console.cloud.google.com/bigquery?p=acme&d=sales&t=orders
sources:
  - resource: https://wiki.acme/finance/revenue-recognition
    title: Revenue recognition policy
---

See the [ingestion job dashboard](https://example.com/dash).
`))
	if err != nil {
		panic(err)
	}

	b := okf.FromConcepts(map[string]*okf.Concept{"tables/orders": c})
	for _, l := range b.ExternalLinks() {
		fmt.Println(l.URL)
	}
	// Output:
	// https://wiki.acme/finance/revenue-recognition
	// https://console.cloud.google.com/bigquery?p=acme&d=sales&t=orders
	// https://example.com/dash
}

// ExampleCanonicalURL normalizes a URL for comparison only: scheme and host
// are lowercased, the default port and fragment are dropped, and the query
// string is preserved.
func ExampleCanonicalURL() {
	fmt.Println(okf.CanonicalURL("HTTPS://Example.com:443/path/?b=2&a=1#frag"))
	// Output:
	// https://example.com/path/?b=2&a=1
}

// ExampleLog_Insert adds an entry to a log.md, with the caller supplying the
// date so rendering stays deterministic.
func ExampleLog_Insert() {
	lg := &okf.Log{}
	lg.Insert(okf.LogEntry{
		Date: okf.NewDate(2026, time.May, 15),
		Kind: "Initialization",
		Text: "Created foundational directory structure.",
	})

	fmt.Print(string(lg.Bytes()))
	// Output:
	// # Directory Update Log
	//
	// ## 2026-05-15
	// * **Initialization**: Created foundational directory structure.
}

// Playbook is a small consumer-defined concept type, registered below to
// demonstrate go-okf's extensibility story: a custom `type` value decoded
// through the package-level registry rather than a closed interface.
type Playbook struct {
	Title string `yaml:"title"`
	Owner string `yaml:"owner"`
}

// ExampleRegister registers a decoder for a custom concept type and
// dispatches to it via Concept.Typed.
func ExampleRegister() {
	okf.Register("Playbook", func(c *okf.Concept) (any, error) {
		return okf.As[Playbook](c)
	})

	c, err := okf.Parse([]byte("---\ntype: Playbook\ntitle: Incident response\nowner: team:sre\n---\n\nbody\n"))
	if err != nil {
		panic(err)
	}

	v, ok := c.Typed()
	if !ok {
		panic("expected Playbook to be registered")
	}

	pb := v.(*Playbook)
	fmt.Println(pb.Title, pb.Owner)
	// Output:
	// Incident response team:sre
}

// ExampleAs decodes a concept's merged frontmatter (core fields plus custom
// keys) straight into a caller-defined struct via yaml tags — the
// compile-time path that needs no registry entry.
func ExampleAs() {
	c, err := okf.Parse([]byte("---\ntype: Playbook\ntitle: Incident response\nowner: team:sre\n---\n\nbody\n"))
	if err != nil {
		panic(err)
	}

	pb, err := okf.As[Playbook](c)
	if err != nil {
		panic(err)
	}

	fmt.Println(pb.Title, pb.Owner)
	// Output:
	// Incident response team:sre
}

// ExampleBundle_RegenerateIndexes synthesizes an index.md for a bundle,
// grouping concepts by type and sorting entries by title. Passing a nil
// [okf.Synthesizer] is fine here: no subdirectory summaries are needed.
func ExampleBundle_RegenerateIndexes() {
	b := okf.FromConcepts(map[string]*okf.Concept{
		"orders":    {Type: "BigQuery Table", Title: "Orders"},
		"customers": {Type: "BigQuery Table", Title: "Customers"},
	})

	indexes := b.RegenerateIndexes(nil)
	fmt.Print(string(indexes[""]))
	// Output:
	// # BigQuery Table
	//
	// * [Customers](customers.md)
	// * [Orders](orders.md)
}

// ExampleConcept_AsAttestedComputation decodes an Attested Computation
// concept (OKF v0.2 §10.2): a sanctioned, re-runnable way to produce a
// value, plus the executor and attester that check it.
func ExampleConcept_AsAttestedComputation() {
	c, err := okf.Parse([]byte(`---
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
`))
	if err != nil {
		panic(err)
	}

	ac, ok := c.AsAttestedComputation()
	if !ok {
		panic("expected an Attested Computation")
	}

	fmt.Println(ac.Runtime)
	fmt.Println(ac.Parameters[0].Name, ac.Parameters[0].Required)
	fmt.Println(ac.Executor.Resource)
	fmt.Println(ac.Attester.Resource)
	// Output:
	// bigquery
	// year true
	// references/skills/run-on-bq.md
	// references/attesters/revenue.py
}

// ExampleBundle_Conformance checks a bundle against OKF v0.2 §11: a concept
// missing its required `type` produces one violation.
func ExampleBundle_Conformance() {
	fsys := fstest.MapFS{
		"tables/orders.md": &fstest.MapFile{Data: []byte("---\ntitle: Orders\n---\n\nbody\n")},
	}
	b, err := okf.Load(fsys)
	if err != nil {
		panic(err)
	}

	for _, v := range b.Conformance() {
		fmt.Println(v.ConceptID, v.Rule)
	}
	// Output:
	// tables/orders type-required
}

// ExampleConcept_TrustTier derives a concept's trust tier from a `verified`
// actor: any `human:` actor makes the concept human-reviewed.
func ExampleConcept_TrustTier() {
	c, err := okf.Parse([]byte(`---
type: Metric
title: Revenue
verified: { by: human:ahormati, at: 2026-06-25T09:00:00Z }
---

body
`))
	if err != nil {
		panic(err)
	}

	fmt.Println(c.TrustTier())
	// Output:
	// human-reviewed
}
