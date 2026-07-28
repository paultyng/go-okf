package okf

import (
	"reflect"
	"strings"
	"testing"
	"time"

	gast "github.com/yuin/goldmark/ast"
)

func TestIsExternalTarget(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   bool
	}{
		{"bundle-relative absolute path", "/x.md", false},
		{"bundle-relative dotted path", "./x.md", false},
		{"absolute https URL", "https://example.com/x.md", true},
		{"protocol-relative (no scheme, treated as internal)", "//host/x", false},
		{"mailto scheme", "mailto:someone@example.com", true},
		{"invalid percent-encoding (url.Parse error)", "%zz", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isExternalTarget(tt.target); got != tt.want {
				t.Errorf("isExternalTarget(%q) = %v, want %v", tt.target, got, tt.want)
			}
		})
	}
}

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

// Regression tests for nodeText's replacement of the deprecated,
// type-specific ast.Node.Text: AutoLink, String, and RawHTML store their
// text in private fields rather than child *ast.Text nodes, so a nodeText
// that only recurses through FirstChild silently drops their content.

func TestExtractLinksAutoLinkAsLinkText(t *testing.T) {
	c := &Concept{Body: "[<https://inner>](https://target)\n"}
	links := c.Links()
	if len(links) != 1 {
		t.Fatalf("len(Links()) = %d, want 1: %+v", len(links), links)
	}
	if links[0].Text != "https://inner" {
		t.Errorf("Text = %q, want the autolink's label preserved as the link text", links[0].Text)
	}
	if links[0].Target != "https://target" {
		t.Errorf("Target = %q, want %q", links[0].Target, "https://target")
	}
}

func TestExtractCitationsBareAutoLinkURL(t *testing.T) {
	c := &Concept{Body: "# Citations\n- <https://example.com/auto>\n"}
	citations := c.Citations()
	if len(citations) != 1 {
		t.Fatalf("len(Citations()) = %d, want 1: %+v", len(citations), citations)
	}
	if citations[0].URL != "https://example.com/auto" {
		t.Errorf("URL = %q, want the autolink URL preserved", citations[0].URL)
	}
}

func TestExtractCitationsRawHTMLInLinkTextPreserved(t *testing.T) {
	c := &Concept{Body: "# Citations\n- [Raw <br> html](https://example.com/x)\n"}
	citations := c.Citations()
	if len(citations) != 1 {
		t.Fatalf("len(Citations()) = %d, want 1: %+v", len(citations), citations)
	}
	if citations[0].Title != "Raw <br> html" {
		t.Errorf("Title = %q, want the raw HTML tag preserved verbatim in the link text", citations[0].Title)
	}
}

func TestNodeTextPreservesRawHTMLInHeading(t *testing.T) {
	doc, source := parseBody(extractMarkdown, "# Raw <br> html\n")
	var heading *gast.Heading
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		if h, ok := n.(*gast.Heading); ok {
			heading = h
			return gast.WalkStop, nil
		}
		return gast.WalkContinue, nil
	})
	if heading == nil {
		t.Fatal("expected to find a heading node")
	}
	if got := string(nodeText(heading, source)); got != "Raw <br> html" {
		t.Errorf("nodeText(heading) = %q, want %q", got, "Raw <br> html")
	}
}

// TestExtractCitationsFootnoteMarkerParsedLiterally locks in Fix 2:
// Citations/Links must parse with the footnote-free config, matching
// git-show HEAD (pre-refactor) go-okf. This body is an adversarial edge
// case (a "[^1]" marker nested inside a citation's link text, with a
// same-named "[^1]: def" line elsewhere) chosen because it's one of the
// few inputs where the footnote-vs-footnote-free parser choice visibly
// changes Citations() output: without the Footnote extension, "[^1]: def"
// is read as an ordinary CommonMark link-reference-definition, so the
// nested "[^1]" resolves as its own link (a pre-existing, if obscure,
// go-okf behavior we must not regress) — Title/URL below are exactly what
// HEAD's original two-parser code produces for this body, confirmed by
// running it against git-show HEAD:extract.go directly. The single-parser
// refactor this fix undoes instead treated "[^1]" as a footnote reference,
// producing {Title: URL:Revenue recognition} for the same body.
func TestExtractCitationsFootnoteMarkerParsedLiterally(t *testing.T) {
	c := &Concept{Body: "# Citations\n- [Revenue recognition[^1]](https://example.com/rev)\n\n[^1]: def\n"}
	citations := c.Citations()
	if len(citations) != 1 {
		t.Fatalf("len(Citations()) = %d, want 1: %+v", len(citations), citations)
	}
	if citations[0].Title != "^1" || citations[0].URL != "def" {
		t.Errorf("Citations()[0] = %+v, want {Title:^1 URL:def} (footnote-free parse, matching pre-refactor HEAD)", citations[0])
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

// TestMaxParseBytesBoundsAdversarialBody guards against goldmark's
// superlinear parse cost on adversarially nested markdown (see
// MaxParseBytes's doc comment). Deeply nested blockquote markers are a
// known trigger: a 300KB body of nothing but ">" characters measurably
// fails to parse within a 5s budget with no cap in place. With a small
// cap, extraction on a much larger (5MB) adversarial body must still
// complete quickly, because only the first MaxParseBytes bytes are ever
// handed to the parser.
func TestMaxParseBytesBoundsAdversarialBody(t *testing.T) {
	orig := MaxParseBytes
	MaxParseBytes = 4096
	defer func() { MaxParseBytes = orig }()

	body := strings.Repeat(">", 5_000_000) + " x"
	c := &Concept{Body: body}

	done := make(chan []Link, 1)
	go func() { done <- c.Links() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Links() did not return within the time budget; MaxParseBytes cap did not bound the parse")
	}

	if c.Body != body {
		t.Error("Concept.Body was mutated by extraction, want the cap to only bound the parser's input, never the stored body")
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

func TestBundleCitationsAggregatesAcrossConcepts(t *testing.T) {
	b := FromConcepts(map[string]*Concept{
		"b": {Type: "X", Body: "# Citations\n- https://example.com/b1\n"},
		"a": {Type: "X", Body: "# Citations\n- https://example.com/a1\n- https://example.com/a2\n"},
	})
	got := b.Citations()
	if len(got) != 3 {
		t.Fatalf("Citations() = %+v, want 3 entries across both concepts", got)
	}
	// sortedIDs visits "a" before "b", so a's citations come first.
	if got[0].URL != "https://example.com/a1" || got[1].URL != "https://example.com/a2" {
		t.Errorf("Citations()[0:2] = %+v, want a's citations in id order", got[:2])
	}
	if got[2].URL != "https://example.com/b1" {
		t.Errorf("Citations()[2] = %+v, want b's citation", got[2])
	}
}
