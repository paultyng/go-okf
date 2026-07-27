package okf

import (
	"strconv"
	"strings"
	"testing"
)

func TestIndexParseAndBytesRoundTrip(t *testing.T) {
	src := []byte(`---
okf_version: "0.2"
---

# BigQuery Table

* [events_*](events_.md) - Daily-sharded GA4 event tables.
* [users](users.md) - Per-user dimension.

# Subdirectories

* [datasets](datasets/index.md) - GA4 obfuscated ecommerce sample.
`)
	idx, err := ParseIndex(src)
	if err != nil {
		t.Fatalf("ParseIndex: %v", err)
	}
	if idx.OKFVersion != "0.2" {
		t.Errorf("OKFVersion = %q", idx.OKFVersion)
	}
	if len(idx.Sections) != 2 {
		t.Fatalf("len(Sections) = %d: %+v", len(idx.Sections), idx.Sections)
	}
	if idx.Sections[0].Heading != "BigQuery Table" || len(idx.Sections[0].Entries) != 2 {
		t.Errorf("Sections[0] = %+v", idx.Sections[0])
	}
	if idx.Sections[0].Entries[0].Title != "events_*" || idx.Sections[0].Entries[0].Link != "events_.md" {
		t.Errorf("Sections[0].Entries[0] = %+v", idx.Sections[0].Entries[0])
	}

	reparsed, err := ParseIndex(idx.Bytes())
	if err != nil {
		t.Fatalf("re-ParseIndex: %v", err)
	}
	if reparsed.OKFVersion != idx.OKFVersion || len(reparsed.Sections) != len(idx.Sections) {
		t.Errorf("round-trip mismatch: %+v vs %+v", reparsed, idx)
	}
}

func TestIndexNoFrontmatterForNonRoot(t *testing.T) {
	src := []byte("# BigQuery Table\n\n* [x](x.md) - desc\n")
	idx, err := ParseIndex(src)
	if err != nil {
		t.Fatalf("ParseIndex: %v", err)
	}
	if idx.OKFVersion != "" {
		t.Errorf("OKFVersion = %q, want empty", idx.OKFVersion)
	}
	if len(idx.Sections) != 1 || len(idx.Sections[0].Entries) != 1 {
		t.Errorf("Sections = %+v", idx.Sections)
	}
}

func TestBuildIndexSectionsGroupsAndSorts(t *testing.T) {
	entries := []IndexBuildEntry{
		{Type: "BigQuery Table", Title: "users", Link: "users.md"},
		{Type: "BigQuery Table", Title: "events_*", Link: "events_.md"},
		{Type: "", Title: "orphan", Link: "orphan.md"},
	}
	sections := BuildIndexSections(entries)
	if len(sections) != 2 {
		t.Fatalf("len(sections) = %d: %+v", len(sections), sections)
	}
	// "BigQuery Table" sorts before "Other".
	if sections[0].Heading != "BigQuery Table" {
		t.Errorf("sections[0].Heading = %q", sections[0].Heading)
	}
	if sections[0].Entries[0].Title != "events_*" || sections[0].Entries[1].Title != "users" {
		t.Errorf("entries not sorted by title: %+v", sections[0].Entries)
	}
	if sections[1].Heading != "Other" {
		t.Errorf("sections[1].Heading = %q, want Other", sections[1].Heading)
	}
}

func TestRegenerateIndexesGroupsByTypeAndLinksRelative(t *testing.T) {
	b := FromConcepts(map[string]*Concept{
		"datasets/ga4":   {Type: "BigQuery Dataset", Title: "GA4 Dataset", Description: "GA4 obfuscated ecommerce sample."},
		"tables/events_": {Type: "BigQuery Table", Title: "events_*", Description: "Daily-sharded GA4 event tables."},
		"tables/users":   {Type: "BigQuery Table", Title: "users", Description: "Per-user dimension."},
	})

	synth := func(dir string, children []IndexChildSummary) string {
		return "stub: " + strconv.Itoa(len(children)) + " items"
	}

	written := b.RegenerateIndexes(synth)

	for _, want := range []string{"", "datasets", "tables"} {
		if _, ok := written[want]; !ok {
			t.Errorf("expected an index.md for directory %q; got keys %v", want, mapKeys(written))
		}
	}

	tablesIndex := string(written["tables"])
	for _, want := range []string{"# BigQuery Table", "[events_*](events_.md)", "[users](users.md)", "Daily-sharded GA4 event tables."} {
		if !strings.Contains(tablesIndex, want) {
			t.Errorf("tables index missing %q:\n%s", want, tablesIndex)
		}
	}

	rootIndex := string(written[""])
	for _, want := range []string{"# Subdirectories", "(datasets/index.md) - GA4 obfuscated ecommerce sample.", "(tables/index.md) - stub: 2 items"} {
		if !strings.Contains(rootIndex, want) {
			t.Errorf("root index missing %q:\n%s", want, rootIndex)
		}
	}
}

func TestRegenerateIndexesSkipsEmptyBundle(t *testing.T) {
	b := FromConcepts(map[string]*Concept{})
	written := b.RegenerateIndexes(nil)
	if len(written) != 0 {
		t.Errorf("RegenerateIndexes() = %v, want none for a bundle with no concepts", written)
	}
}

func TestRegenerateIndexesSingleChildReusesDescription(t *testing.T) {
	b := FromConcepts(map[string]*Concept{
		"datasets/only": {Type: "BigQuery Dataset", Title: "Only Dataset", Description: "The only dataset in this bundle."},
	})

	calls := 0
	synth := func(dir string, children []IndexChildSummary) string {
		calls++
		return "should not be called"
	}

	written := b.RegenerateIndexes(synth)
	rootIndex := string(written[""])
	if !strings.Contains(rootIndex, "(datasets/index.md) - The only dataset in this bundle.") {
		t.Errorf("root index missing reused description:\n%s", rootIndex)
	}
	if calls != 0 {
		t.Errorf("synth called %d times, want 0 (single described child should be reused)", calls)
	}
}

func TestRegenerateIndexesSubdirectoriesSortedAmongTypes(t *testing.T) {
	b := FromConcepts(map[string]*Concept{
		"zebra":      {Type: "Zebra", Title: "Zebra Concept"},
		"sub/animal": {Type: "Animal", Title: "Sub Animal"},
	})

	written := b.RegenerateIndexes(nil)
	rootIndex := string(written[""])

	subIdx := strings.Index(rootIndex, "# Subdirectories")
	zebraIdx := strings.Index(rootIndex, "# Zebra")
	if subIdx == -1 || zebraIdx == -1 {
		t.Fatalf("expected both headings in root index:\n%s", rootIndex)
	}
	if subIdx >= zebraIdx {
		t.Errorf("# Subdirectories must sort before # Zebra alphabetically:\n%s", rootIndex)
	}
}

func mapKeys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
