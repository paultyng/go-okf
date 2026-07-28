package okf

import (
	"reflect"
	"sort"
	"testing"
)

func TestSourceKindString(t *testing.T) {
	tests := []struct {
		kind SourceKind
		want string
	}{
		{SourceResource, "resource"},
		{SourceProvenance, "sources"},
		{SourceCitation, "citation"},
		{SourceInlineLink, "inline-link"},
		{SourceKind(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("SourceKind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestExternalLinksPrecedence(t *testing.T) {
	b := FromConcepts(map[string]*Concept{
		"revenue": {
			Type:     "Attested Computation",
			Title:    "Revenue",
			Resource: "https://wiki.acme/finance/revenue-recognition",
			Sources: []Source{
				{Resource: "https://wiki.acme/finance/revenue-recognition", Title: "Revenue recognition policy"},
			},
			Body: `# Citations
- [Old citation form](https://wiki.acme/finance/revenue-recognition)

See also a fresh [other](https://example.com/only-inline).
`,
		},
	})

	links := b.ExternalLinks()
	var revenue, other *ExternalLink
	for i := range links {
		switch links[i].URL {
		case "https://wiki.acme/finance/revenue-recognition":
			revenue = &links[i]
		case "https://example.com/only-inline":
			other = &links[i]
		}
	}
	if revenue == nil {
		t.Fatal("expected the shared URL to be deduplicated into one ExternalLink")
	}
	if revenue.Label != "Revenue recognition policy" {
		t.Errorf("Label = %q, want the sources[] title to win over resource/citation/inline", revenue.Label)
	}
	if revenue.Type != "Attested Computation" {
		t.Errorf("Type = %q, want the resource-level type", revenue.Type)
	}
	// resource + sources(=SourceProvenance) + citation + inline all point at
	// the same canonical URL, so all four origins should be recorded.
	if len(revenue.Origins) != 4 {
		t.Errorf("Origins = %+v, want 4 discovery sites", revenue.Origins)
	}

	if other == nil {
		t.Fatal("expected the inline-only URL to appear as its own ExternalLink")
	}
	if other.Label != "other" {
		t.Errorf("Label = %q, want the inline link text as fallback label", other.Label)
	}
}

func TestExternalLinksDedupesByCanonicalURL(t *testing.T) {
	b := FromConcepts(map[string]*Concept{
		"a": {Type: "X", Resource: "https://Example.com:443/path#frag"},
		"b": {Type: "Y", Resource: "https://example.com/path"},
	})
	links := b.ExternalLinks()
	if len(links) != 1 {
		t.Fatalf("ExternalLinks() = %+v, want 1 deduplicated entry", links)
	}
	if len(links[0].Origins) != 2 {
		t.Errorf("Origins = %+v, want both concepts recorded", links[0].Origins)
	}
}

func TestExternalLinksWiresFootnoteCitedSourceOrigin(t *testing.T) {
	b := FromConcepts(map[string]*Concept{
		"claim": {
			Type:    "X",
			Sources: []Source{{ID: "s1", Resource: "https://x/y"}},
			Body:    "text[^s1]",
		},
	})

	links := b.ExternalLinks()
	var got *ExternalLink
	for i := range links {
		if links[i].URL == "https://x/y" {
			got = &links[i]
		}
	}
	if got == nil {
		t.Fatal("expected an ExternalLink for https://x/y")
	}

	found := false
	for _, o := range got.Origins {
		if o.ConceptID == "claim" && o.Source == SourceProvenance {
			found = true
		}
	}
	if !found {
		t.Errorf("Origins = %+v, want an Origin{ConceptID: claim, Source: SourceProvenance} for the footnote-cited source", got.Origins)
	}
}

// TestExternalLinksSharedParseBehaviorPreserved exercises all four
// discovery kinds (resource, a footnote-cited source, a legacy citation,
// and a plain inline link) on one concept in a single ExternalLinks() call,
// to confirm the single-shared-parse refactor (extraction no longer
// re-parses the body once per kind) doesn't change the merged result:
// footnote precedence still wins for the shared URL, and every kind's
// metadata still surfaces for its own URL.
func TestExternalLinksSharedParseBehaviorPreserved(t *testing.T) {
	b := FromConcepts(map[string]*Concept{
		"c": {
			Type:     "X",
			Title:    "Concept title",
			Resource: "https://example.com/resource-only",
			Sources: []Source{
				{ID: "s1", Resource: "https://example.com/footnoted", Title: "Footnoted source"},
			},
			Body: "Uses the schema.[^s1]\n\n[^s1]: schema definition\n\n" +
				"# Citations\n- [Old citation](https://example.com/cited)\n\n" +
				"See also [inline](https://example.com/inline-only).\n",
		},
	})

	links := b.ExternalLinks()
	byURL := map[string]ExternalLink{}
	var urls []string
	for _, l := range links {
		byURL[l.URL] = l
		urls = append(urls, l.URL)
	}
	sort.Strings(urls)
	want := []string{
		"https://example.com/cited",
		"https://example.com/footnoted",
		"https://example.com/inline-only",
		"https://example.com/resource-only",
	}
	if !reflect.DeepEqual(urls, want) {
		t.Fatalf("ExternalLinks() URLs = %v, want %v", urls, want)
	}

	if got := byURL["https://example.com/footnoted"].Label; got != "Footnoted source" {
		t.Errorf("footnoted source Label = %q, want the sources[] title", got)
	}
	if got := byURL["https://example.com/cited"].Label; got != "Old citation" {
		t.Errorf("citation Label = %q, want the legacy Citations entry title", got)
	}
	if got := byURL["https://example.com/inline-only"].Label; got != "inline" {
		t.Errorf("inline Label = %q, want the inline link text", got)
	}
	if got := byURL["https://example.com/resource-only"].Type; got != "X" {
		t.Errorf("resource Type = %q, want the concept's type", got)
	}
}

// TestExternalLinksCitationFootnoteMarkerMatchesPreRefactor locks in Fix 2:
// ExternalLinks must extract citations/links with the footnote-free parser
// (2 parses per concept total: one footnote-free for citations+links, one
// footnote-enabled only for FootnoteSourceIDs), matching pre-refactor
// go-okf. Before the fix, ExternalLinks reused one footnote-enabled parser
// for everything, so "[^1]" here was consumed as a footnote reference: the
// citation's link resolution broke (its URL became the literal word
// "Documented", which isn't an external target), and this URL vanished
// from the result entirely. With the footnote-free parser restored, "[^1]:
// https://example.com/from-ref" is read as an ordinary CommonMark
// link-reference-definition and "[^1]" resolves to it, exactly as
// git-show HEAD:extract.go's Citations() produces for this body.
func TestExternalLinksCitationFootnoteMarkerMatchesPreRefactor(t *testing.T) {
	b := FromConcepts(map[string]*Concept{
		"c": {
			Type: "X",
			Body: "# Citations\n- Documented[^1]\n\n[^1]: https://example.com/from-ref\n",
		},
	})
	links := b.ExternalLinks()
	if len(links) != 1 || links[0].URL != "https://example.com/from-ref" {
		t.Fatalf("ExternalLinks() = %+v, want a single entry for https://example.com/from-ref", links)
	}
}

func TestExternalLinksIgnoresBundleRelative(t *testing.T) {
	b := FromConcepts(map[string]*Concept{
		"a": {Type: "X", Resource: "/tables/customers.md"},
	})
	if got := b.ExternalLinks(); len(got) != 0 {
		t.Errorf("ExternalLinks() = %+v, want none for a bundle-relative resource", got)
	}
}
