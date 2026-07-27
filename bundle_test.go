package okf

import (
	"testing"
	"testing/fstest"
)

func TestLoadMapFS(t *testing.T) {
	fsys := fstest.MapFS{
		"index.md":            &fstest.MapFile{Data: []byte("---\nokf_version: \"0.2\"\n---\n\n# Tables\n\n* [Customers](tables/customers.md)\n")},
		"log.md":              &fstest.MapFile{Data: []byte("# Directory Update Log\n\n## 2026-05-15\n* **Initialization**: Created foundational directory structure.\n")},
		"tables/customers.md": &fstest.MapFile{Data: []byte("---\ntype: BigQuery Table\ntitle: Customers\n---\n\nbody\n")},
		"tables/orders.md":    &fstest.MapFile{Data: []byte("---\ntype: BigQuery Table\ntitle: Orders\n---\n\nSee [customers](/tables/customers.md).\n")},
		"broken.md":           &fstest.MapFile{Data: []byte("not frontmatter at all, just prose")},
	}

	b, err := Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := len(b.Concepts()); got != 3 {
		t.Fatalf("len(Concepts()) = %d, want 3 (customers, orders, broken-as-no-frontmatter)", got)
	}

	c, ok := b.Concept("tables/customers")
	if !ok {
		t.Fatal("expected tables/customers to be loaded")
	}
	if c.Title != "Customers" {
		t.Errorf("Title = %q", c.Title)
	}

	if _, ok := b.Concept("broken"); !ok {
		t.Error("a file with no frontmatter should still load as a bodyless-frontmatter concept, not be dropped")
	}

	idx, ok := b.Index("")
	if !ok {
		t.Fatal("expected a root index.md to be loaded")
	}
	if idx.OKFVersion != "0.2" {
		t.Errorf("OKFVersion = %q", idx.OKFVersion)
	}

	lg, ok := b.Log("")
	if !ok {
		t.Fatal("expected a root log.md to be loaded")
	}
	if len(lg.Sections) != 1 || lg.Sections[0].Entries[0].Kind != "Initialization" {
		t.Errorf("Log.Sections = %+v", lg.Sections)
	}
}

func TestLoadTolerantOfUnknownTypes(t *testing.T) {
	fsys := fstest.MapFS{
		"weird.md": &fstest.MapFile{Data: []byte("---\ntype: SomeNeverBeforeSeenType\n---\n\nbody\n")},
	}
	b, err := Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := b.Concept("weird"); !ok {
		t.Error("expected an unknown type to load, not be rejected")
	}
}

func TestConceptIDStrictVsPermissive(t *testing.T) {
	if _, err := ParseConceptID("tables/customers"); err != nil {
		t.Errorf("ParseConceptID(valid) errored: %v", err)
	}
	if _, err := ParseConceptID(""); err == nil {
		t.Error("expected an error for an empty concept id")
	}
	if _, err := ParseConceptID("tables/*bad*"); err == nil {
		t.Error("expected an error for an invalid segment")
	}
	if _, err := ParseConceptID("tables/has space"); err == nil {
		t.Error("expected an error for a segment containing a space")
	}

	// ConceptIDFromPath is permissive: it accepts whatever is on disk.
	if got := ConceptIDFromPath("tables/has space.md"); got != ConceptID("tables/has space") {
		t.Errorf("ConceptIDFromPath = %q", got)
	}
	if got := ConceptIDFromPath("root.md"); got != ConceptID("root") {
		t.Errorf("ConceptIDFromPath = %q", got)
	}
}

func TestLoadTolerantOfBadStaleAfter(t *testing.T) {
	fsys := fstest.MapFS{
		"good.md": &fstest.MapFile{Data: []byte("---\ntype: X\nstale_after: 2026-09-23\n---\n\nbody\n")},
		"bad.md":  &fstest.MapFile{Data: []byte("---\ntype: X\nstale_after: not-a-date\n---\n\nbody\n")},
	}

	b, err := Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := len(b.Concepts()); got != 2 {
		t.Fatalf("len(Concepts()) = %d, want 2 (an unparseable stale_after must not drop the concept)", got)
	}

	bad, ok := b.Concept("bad")
	if !ok {
		t.Fatal("expected the concept with a bad stale_after to still load")
	}
	if bad.IsStale(NewDate(2099, 1, 1)) {
		t.Error("IsStale() = true, want false for an unparseable/absent stale_after")
	}
}

func TestFromConcepts(t *testing.T) {
	b := FromConcepts(map[string]*Concept{
		"a": {Type: "X"},
		"b": {Type: "Y"},
	})
	if len(b.Concepts()) != 2 {
		t.Fatalf("len(Concepts()) = %d", len(b.Concepts()))
	}
	if _, ok := b.Concept("a"); !ok {
		t.Error("expected concept a")
	}
}
