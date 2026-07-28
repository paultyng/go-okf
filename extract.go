package okf

import (
	"bytes"
	"net/url"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	fnast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// MaxParseBytes bounds how many bytes of a [Concept.Body] are handed to the
// markdown parser during extraction (Links, Citations, FootnoteSourceIDs,
// Bundle.ExternalLinks, and the Attested Computation `# Computation` fence).
// goldmark's parser can be superlinear on adversarially nested markdown
// (deeply nested lists, blockquotes, emphasis runs), and Body is untrusted
// producer input with no other size bound in this package.
//
// The stored Concept.Body is never mutated or truncated by this cap — only
// the bytes fed to the parser are bounded. A body longer than MaxParseBytes
// still round-trips and marshals in full; only extraction sees a truncated
// view, and so may miss a link, citation, footnote, or fence that starts
// beyond the cutoff.
//
// The default (1 MiB) is generous, sized to avoid truncating legitimate
// documents. Consumers that feed fully-untrusted, adversarial input to this
// package should lower MaxParseBytes considerably. Set to 0 to disable the
// cap entirely.
var MaxParseBytes = 1 << 20

// extractMarkdown is the footnote-free goldmark configuration used for
// Links and Citations extraction (and any other site that walks a body for
// literal link/citation text, e.g. log.go). It must not enable the
// Footnote extension: a `[^id]` marker inside link or citation text needs
// to stay literal text, not become a footnote reference node, matching
// pre-existing producer content that never anticipated footnotes.
var extractMarkdown = goldmark.New()

// footnoteMarkdown is the goldmark configuration used only for
// FootnoteSourceIDs, which needs `[^id]` markers recognized as footnote
// references rather than literal text.
var footnoteMarkdown = goldmark.New(goldmark.WithExtensions(extension.Footnote))

// parseBody parses body into a goldmark AST using md, bounding the bytes fed
// to the parser by [MaxParseBytes]. It never mutates or truncates the
// caller's body string — the returned source is a (possibly shorter) slice
// used only for this parse.
func parseBody(md goldmark.Markdown, body string) (gast.Node, []byte) {
	source := []byte(body)
	if MaxParseBytes > 0 && len(source) > MaxParseBytes {
		source = source[:MaxParseBytes]
	}
	return md.Parser().Parse(text.NewReader(source)), source
}

// nodeText concatenates the literal text of n's descendants, honoring soft
// line breaks. It is the non-deprecated equivalent of ast.Node.Text
// (SA1019: deprecated in favor of node-specific accessors), and must match
// that deprecated method's per-kind behavior: some leaf kinds (AutoLink,
// String, RawHTML) hold their text in private fields rather than children,
// so recursing into FirstChild alone silently drops their text.
func nodeText(n gast.Node, source []byte) []byte {
	var buf bytes.Buffer
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch t := c.(type) {
		case *gast.Text:
			buf.Write(t.Value(source))
			if t.SoftLineBreak() {
				buf.WriteByte('\n')
			}
		case *gast.AutoLink:
			buf.Write(t.Label(source))
		case *gast.String:
			buf.Write(t.Value)
		case *gast.RawHTML:
			buf.Write(t.Segments.Value(source))
		default:
			buf.Write(nodeText(c, source))
		}
	}
	return buf.Bytes()
}

// Link is a markdown link found in a concept body (OKF v0.2 §6.1).
// External is false for a bundle-relative link (absolute "/..." or
// relative "./...", "../...").
type Link struct {
	Text     string
	Target   string
	External bool
}

// Citation is a legacy v0.1 body `# Citations` list entry (superseded by
// `sources` frontmatter in v0.2 — see OKF v0.2 §13.1). Index is the
// 1-based position within the list.
type Citation struct {
	Index int
	Title string
	URL   string
}

// isExternalTarget reports whether a link target is an absolute URL (has a
// scheme) as opposed to a bundle-relative path.
func isExternalTarget(target string) bool {
	u, err := url.Parse(target)
	if err != nil {
		return false
	}
	return u.IsAbs()
}

// Links returns every markdown link in the concept's body, each flagged
// internal (bundle-relative) or external.
func (c *Concept) Links() []Link {
	doc, source := parseBody(extractMarkdown, c.Body)
	return extractLinks(doc, source)
}

func extractLinks(doc gast.Node, source []byte) []Link {
	var links []Link
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		if l, ok := n.(*gast.Link); ok {
			target := string(l.Destination)
			links = append(links, Link{
				Text:     string(nodeText(l, source)),
				Target:   target,
				External: isExternalTarget(target),
			})
		}
		return gast.WalkContinue, nil
	})
	return links
}

// Citations returns the legacy v0.1 `# Citations` body list, if present.
// v0.2 producers should prefer `sources` frontmatter ([Concept].Sources);
// this is a fallback for reading older documents (§13.1).
func (c *Concept) Citations() []Citation {
	doc, source := parseBody(extractMarkdown, c.Body)
	return extractCitations(doc, source)
}

func extractCitations(doc gast.Node, source []byte) []Citation {
	var citations []Citation
	inSection := false
	sectionLevel := 0
	idx := 0

	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		if h, ok := n.(*gast.Heading); ok {
			if inSection {
				if h.Level <= sectionLevel {
					return gast.WalkStop, nil
				}
				return gast.WalkSkipChildren, nil
			}
			if strings.EqualFold(strings.TrimSpace(string(nodeText(h, source))), "Citations") {
				inSection = true
				sectionLevel = h.Level
			}
			return gast.WalkSkipChildren, nil
		}
		if inSection {
			if list, ok := n.(*gast.List); ok {
				for item := list.FirstChild(); item != nil; item = item.NextSibling() {
					li, ok := item.(*gast.ListItem)
					if !ok {
						continue
					}
					idx++
					citations = append(citations, parseCitationItem(li, source, idx))
				}
				return gast.WalkSkipChildren, nil
			}
		}
		return gast.WalkContinue, nil
	})
	return citations
}

func parseCitationItem(li gast.Node, source []byte, idx int) Citation {
	cit := Citation{Index: idx}

	var link *gast.Link
	_ = gast.Walk(li, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		if l, ok := n.(*gast.Link); ok && link == nil {
			link = l
			return gast.WalkSkipChildren, nil
		}
		return gast.WalkContinue, nil
	})

	if link != nil {
		cit.Title = string(nodeText(link, source))
		cit.URL = string(link.Destination)
		return cit
	}

	cit.URL = strings.TrimSpace(string(nodeText(li, source)))
	return cit
}

// FootnoteSourceIDs returns the sources[].id values that are referenced by
// a markdown footnote (`[^id]`) in the body, sorted — OKF v0.2 §5.1
// per-claim attribution. Unmatched footnotes (whose label has no
// corresponding sources[].id) are ignored, permissively.
func (c *Concept) FootnoteSourceIDs() []string {
	doc, _ := parseBody(footnoteMarkdown, c.Body)
	return footnoteSourceIDs(c.Sources, doc)
}

func footnoteSourceIDs(sources []Source, doc gast.Node) []string {
	sourceIDs := map[string]bool{}
	for _, s := range sources {
		if s.ID != "" {
			sourceIDs[s.ID] = true
		}
	}
	if len(sourceIDs) == 0 {
		return nil
	}

	labelByIndex := map[int]string{}
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		if fn, ok := n.(*fnast.Footnote); ok {
			labelByIndex[fn.Index] = string(fn.Ref)
		}
		return gast.WalkContinue, nil
	})

	seen := map[string]bool{}
	var refs []string
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		if link, ok := n.(*fnast.FootnoteLink); ok {
			label := labelByIndex[link.Index]
			if label != "" && sourceIDs[label] && !seen[label] {
				seen[label] = true
				refs = append(refs, label)
			}
		}
		return gast.WalkContinue, nil
	})
	sort.Strings(refs)
	return refs
}

// Links returns every markdown link across every concept in the bundle.
func (b *Bundle) Links() []Link {
	var out []Link
	for _, id := range b.sortedIDs() {
		out = append(out, b.concepts[ConceptID(id)].Links()...)
	}
	return out
}

// Sources returns the §5.1 provenance entries across every concept.
func (b *Bundle) Sources() []Source {
	var out []Source
	for _, id := range b.sortedIDs() {
		out = append(out, b.concepts[ConceptID(id)].Sources...)
	}
	return out
}

// Citations returns the legacy `# Citations` entries across every concept.
func (b *Bundle) Citations() []Citation {
	var out []Citation
	for _, id := range b.sortedIDs() {
		out = append(out, b.concepts[ConceptID(id)].Citations()...)
	}
	return out
}

// Resources returns the frontmatter `resource:` values across every
// concept, in concept-id order, skipping concepts with no resource set.
func (b *Bundle) Resources() []string {
	var out []string
	for _, id := range b.sortedIDs() {
		if r := b.concepts[ConceptID(id)].Resource; r != "" {
			out = append(out, r)
		}
	}
	return out
}
