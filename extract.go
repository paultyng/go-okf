package okf

import (
	"net/url"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	fnast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

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

var extractMarkdown = goldmark.New()
var footnoteMarkdown = goldmark.New(goldmark.WithExtensions(extension.Footnote))

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
	return extractLinks([]byte(c.Body))
}

func extractLinks(source []byte) []Link {
	doc := extractMarkdown.Parser().Parse(text.NewReader(source))
	var links []Link
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		if l, ok := n.(*gast.Link); ok {
			target := string(l.Destination)
			links = append(links, Link{
				Text:     string(l.Text(source)),
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
	return extractCitations([]byte(c.Body))
}

func extractCitations(source []byte) []Citation {
	doc := extractMarkdown.Parser().Parse(text.NewReader(source))
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
			if strings.EqualFold(strings.TrimSpace(string(h.Text(source))), "Citations") {
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
		cit.Title = string(link.Text(source))
		cit.URL = string(link.Destination)
		return cit
	}

	cit.URL = strings.TrimSpace(string(li.Text(source)))
	return cit
}

// FootnoteSourceIDs returns the sources[].id values that are referenced by
// a markdown footnote (`[^id]`) in the body, sorted — OKF v0.2 §5.1
// per-claim attribution. Unmatched footnotes (whose label has no
// corresponding sources[].id) are ignored, permissively.
func (c *Concept) FootnoteSourceIDs() []string {
	sourceIDs := map[string]bool{}
	for _, s := range c.Sources {
		if s.ID != "" {
			sourceIDs[s.ID] = true
		}
	}
	if len(sourceIDs) == 0 {
		return nil
	}

	source := []byte(c.Body)
	doc := footnoteMarkdown.Parser().Parse(text.NewReader(source))

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
