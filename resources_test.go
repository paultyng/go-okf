package okf

import "testing"

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

func TestExternalLinksIgnoresBundleRelative(t *testing.T) {
	b := FromConcepts(map[string]*Concept{
		"a": {Type: "X", Resource: "/tables/customers.md"},
	})
	if got := b.ExternalLinks(); len(got) != 0 {
		t.Errorf("ExternalLinks() = %+v, want none for a bundle-relative resource", got)
	}
}
