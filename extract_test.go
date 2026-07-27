package okf

import (
	"reflect"
	"testing"
)

func TestExtractLinks(t *testing.T) {
	c := &Concept{Body: "See [customers](/tables/customers.md) and [the docs](https://example.com/docs) and [relative](./other.md).\n"}
	links := c.Links()
	if len(links) != 3 {
		t.Fatalf("len(Links()) = %d, want 3: %+v", len(links), links)
	}
	want := []Link{
		{Text: "customers", Target: "/tables/customers.md", External: false},
		{Text: "the docs", Target: "https://example.com/docs", External: true},
		{Text: "relative", Target: "./other.md", External: false},
	}
	if !reflect.DeepEqual(links, want) {
		t.Errorf("Links() = %+v, want %+v", links, want)
	}
}

func TestExtractCitationsLegacy(t *testing.T) {
	c := &Concept{Body: `# Definition

Some prose.

# Citations
- https://wiki.acme/finance/fpa-handbook
- [Revenue recognition policy](https://wiki.acme/finance/revenue-recognition)
`}
	citations := c.Citations()
	if len(citations) != 2 {
		t.Fatalf("len(Citations()) = %d, want 2: %+v", len(citations), citations)
	}
	if citations[0].Index != 1 || citations[0].URL != "https://wiki.acme/finance/fpa-handbook" {
		t.Errorf("citations[0] = %+v", citations[0])
	}
	if citations[1].Index != 2 || citations[1].Title != "Revenue recognition policy" {
		t.Errorf("citations[1] = %+v", citations[1])
	}
}

func TestExtractCitationsAbsentWhenNoSection(t *testing.T) {
	c := &Concept{Body: "# Definition\n\nNo citations here.\n"}
	if got := c.Citations(); len(got) != 0 {
		t.Errorf("Citations() = %+v, want none", got)
	}
}

func TestFootnoteSourceIDs(t *testing.T) {
	c := &Concept{
		Sources: []Source{
			{ID: "ga4-schema", Resource: "https://developers.google.com/x"},
			{ID: "unused-source", Resource: "https://example.com/y"},
		},
		Body: "The `events_` table is sharded daily.[^ga4-schema]\n\n[^ga4-schema]: GA4 BigQuery Export schema\n",
	}
	got := c.FootnoteSourceIDs()
	if !reflect.DeepEqual(got, []string{"ga4-schema"}) {
		t.Errorf("FootnoteSourceIDs() = %v, want [ga4-schema]", got)
	}
}

func TestFootnoteSourceIDsUnmatchedIgnored(t *testing.T) {
	c := &Concept{
		Sources: []Source{{ID: "a", Resource: "https://example.com/a"}},
		Body:    "A claim.[^not-a-source]\n\n[^not-a-source]: Some footnote unrelated to sources\n",
	}
	if got := c.FootnoteSourceIDs(); len(got) != 0 {
		t.Errorf("FootnoteSourceIDs() = %v, want none", got)
	}
}

func TestBundleAggregatesAcrossConcepts(t *testing.T) {
	b := FromConcepts(map[string]*Concept{
		"a": {Type: "X", Resource: "https://example.com/a", Body: "[link](https://example.com/b)"},
		"b": {Type: "Y", Sources: []Source{{Resource: "https://example.com/c"}}},
	})
	if got := b.Resources(); !reflect.DeepEqual(got, []string{"https://example.com/a"}) {
		t.Errorf("Resources() = %v", got)
	}
	if got := b.Sources(); len(got) != 1 {
		t.Errorf("Sources() = %v", got)
	}
	if got := b.Links(); len(got) != 1 {
		t.Errorf("Links() = %v", got)
	}
}
