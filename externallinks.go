package okf

// SourceKind identifies which channel discovered an external link.
type SourceKind int

const (
	// SourceResource is the frontmatter `resource:` field.
	SourceResource SourceKind = iota
	// SourceProvenance is a `sources[]` frontmatter entry (§5.1).
	SourceProvenance
	// SourceCitation is a legacy body `# Citations` entry (§13.1).
	SourceCitation
	// SourceInlineLink is an ordinary body markdown link.
	SourceInlineLink
)

func (k SourceKind) String() string {
	switch k {
	case SourceResource:
		return "resource"
	case SourceProvenance:
		return "sources"
	case SourceCitation:
		return "citation"
	case SourceInlineLink:
		return "inline-link"
	default:
		return "unknown"
	}
}

// Origin records one site where an [ExternalLink] was discovered.
type Origin struct {
	ConceptID string
	Source    SourceKind
}

// ExternalLink is one external URL referenced anywhere in a bundle,
// deduplicated by [CanonicalURL], with every discovery site recorded.
type ExternalLink struct {
	URL     string // verbatim as first authored — never canonicalized.
	Origins []Origin
	Type    string // optional, from a resource/sources entry.
	Label   string // optional, from a title/sources entry.
}

// externalLinkAcc accumulates state for one canonical URL while merging.
type externalLinkAcc struct {
	firstSeen string
	origins   []Origin
	typ       string
	label     string
	// rank tracks the precedence of the richest metadata source seen so
	// far: lower is higher precedence (resource/sources=0, citation=1,
	// inline=2). Metadata is only overwritten by a strictly higher
	// precedence (lower rank) source.
	rank int
}

const noRank = 1<<31 - 1

// ExternalLinks merges every external reference across the bundle —
// frontmatter `resource:`, `sources[].resource`, legacy body `# Citations`,
// and inline body links — into one deduplicated list keyed by
// [CanonicalURL]. Metadata precedence, richest wins: resource/sources >
// citation > inline link. Origins collect every discovery site.
func (b *Bundle) ExternalLinks() []ExternalLink {
	order := []string{}
	acc := map[string]*externalLinkAcc{}

	upsert := func(id, rawURL, typ, label string, kind SourceKind, rank int) {
		if rawURL == "" || !isExternalTarget(rawURL) {
			return
		}
		canon := CanonicalURL(rawURL)
		a, ok := acc[canon]
		if !ok {
			a = &externalLinkAcc{firstSeen: rawURL, rank: noRank}
			acc[canon] = a
			order = append(order, canon)
		}
		a.origins = append(a.origins, Origin{ConceptID: id, Source: kind})
		if rank < a.rank {
			a.rank = rank
			if typ != "" {
				a.typ = typ
			}
			if label != "" {
				a.label = label
			}
		} else {
			if a.typ == "" && typ != "" {
				a.typ = typ
			}
			if a.label == "" && label != "" {
				a.label = label
			}
		}
	}

	for _, id := range b.sortedIDs() {
		c := b.concepts[ConceptID(id)]

		// Parse the body once with the footnote-free config and reuse the
		// AST for both citations and inline links: `[^id]` markers inside
		// link/citation text must stay literal, matching the pre-footnote
		// extraction behavior those two share. A second, footnote-enabled
		// parse is needed only for footnote-cited source IDs below, since
		// that's the one signal that requires the Footnote extension.
		doc, source := parseBody(extractMarkdown, c.Body)
		footnoteDoc, _ := parseBody(footnoteMarkdown, c.Body)

		// A source cited by a body footnote (`[^id]` matching sources[].id,
		// §4.2 per-claim attribution) is a richer provenance signal than a
		// merely-listed, uncited source: its metadata takes precedence over
		// an uncited source's for the same URL.
		footnoted := map[string]bool{}
		for _, fid := range footnoteSourceIDs(c.Sources, footnoteDoc) {
			footnoted[fid] = true
		}

		// sources[] entries are processed before resource: within their
		// shared precedence tier, a source's specific curated title should
		// win over the concept's own generic title for the same URL.
		for _, s := range c.Sources {
			rank := 0
			if s.ID != "" && footnoted[s.ID] {
				rank = -1
			}
			upsert(id, s.Resource, "", s.Title, SourceProvenance, rank)
		}
		if c.Resource != "" {
			upsert(id, c.Resource, c.Type, c.Title, SourceResource, 0)
		}
		for _, cit := range extractCitations(doc, source) {
			upsert(id, cit.URL, "", cit.Title, SourceCitation, 1)
		}
		for _, l := range extractLinks(doc, source) {
			if l.External {
				upsert(id, l.Target, "", l.Text, SourceInlineLink, 2)
			}
		}
	}

	out := make([]ExternalLink, 0, len(order))
	for _, canon := range order {
		a := acc[canon]
		out = append(out, ExternalLink{URL: a.firstSeen, Origins: a.origins, Type: a.typ, Label: a.label})
	}
	return out
}
