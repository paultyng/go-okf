package okf

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// IndexEntry is one bullet-list item in an index.md section (OKF v0.2 §8):
// `* [Title](Link) - Description`.
type IndexEntry struct {
	Title       string
	Link        string
	Description string
}

// IndexSection is one heading-delimited group of entries in an index.md.
type IndexSection struct {
	Heading string
	Entries []IndexEntry
}

// Index is a parsed (or synthesized) index.md: an untyped directory
// listing for progressive disclosure (OKF v0.2 §8). OKFVersion is only
// ever populated for a bundle-root index.md, the sole place frontmatter is
// permitted in an index.md (§12).
type Index struct {
	OKFVersion string
	Sections   []IndexSection
}

var (
	indexHeadingRe = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
	indexItemRe    = regexp.MustCompile(`^[*-]\s+\[([^\]]*)\]\(([^)]*)\)(?:\s*-\s*(.*))?$`)
)

// ParseIndex parses an index.md's raw bytes. A bundle-root index.md may
// carry a leading YAML frontmatter block whose only recognized key is
// `okf_version`; any other index.md has no frontmatter.
func ParseIndex(data []byte) (*Index, error) {
	text := string(data)
	body := text
	var version string

	if strings.HasPrefix(strings.TrimLeft(text, " \t"), frontmatterDelim) {
		fmBytes, b, hasFM, err := splitFrontmatter(text)
		if err != nil {
			return nil, fmt.Errorf("okf: parsing index.md frontmatter: %w", err)
		}
		if hasFM {
			var fm map[string]any
			if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
				return nil, fmt.Errorf("okf: invalid YAML in index.md frontmatter: %w", err)
			}
			if v, ok := fm["okf_version"]; ok {
				version = fmt.Sprintf("%v", v)
			}
			body = b
		}
	}

	return &Index{OKFVersion: version, Sections: parseIndexSections(body)}, nil
}

func parseIndexSections(body string) []IndexSection {
	var sections []IndexSection
	var cur *IndexSection

	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, " \t\r")
		if line == "" {
			continue
		}
		if m := indexHeadingRe.FindStringSubmatch(line); m != nil {
			sections = append(sections, IndexSection{Heading: m[2]})
			cur = &sections[len(sections)-1]
			continue
		}
		if m := indexItemRe.FindStringSubmatch(line); m != nil {
			if cur == nil {
				sections = append(sections, IndexSection{})
				cur = &sections[len(sections)-1]
			}
			cur.Entries = append(cur.Entries, IndexEntry{
				Title:       m[1],
				Link:        m[2],
				Description: strings.TrimSpace(m[3]),
			})
		}
	}
	return sections
}

// Bytes renders the index back to markdown: a bundle-root okf_version
// frontmatter block (if set) followed by heading-delimited sections.
func (idx *Index) Bytes() []byte {
	var b strings.Builder
	if idx.OKFVersion != "" {
		fmt.Fprintf(&b, "---\nokf_version: %q\n---\n\n", idx.OKFVersion)
	}
	for i, sec := range idx.Sections {
		fmt.Fprintf(&b, "# %s\n\n", sec.Heading)
		for _, e := range sec.Entries {
			if e.Description != "" {
				fmt.Fprintf(&b, "* [%s](%s) - %s\n", e.Title, e.Link, e.Description)
			} else {
				fmt.Fprintf(&b, "* [%s](%s)\n", e.Title, e.Link)
			}
		}
		if i != len(idx.Sections)-1 {
			b.WriteString("\n")
		}
	}
	return []byte(b.String())
}

// IndexBuildEntry is one concept to include when synthesizing an
// index.md section list via BuildIndexSections.
type IndexBuildEntry struct {
	Type        string
	Title       string
	Link        string
	Description string
}

// BuildIndexSections groups entries by Type (an empty Type groups under
// "Other"), sorted by type name, with each section's entries sorted by
// title (case-insensitive) — mirroring the reference synthesizer's
// grouping and sort behavior.
func BuildIndexSections(entries []IndexBuildEntry) []IndexSection {
	grouped := map[string][]IndexEntry{}
	for _, e := range entries {
		typ := e.Type
		if typ == "" {
			typ = "Other"
		}
		grouped[typ] = append(grouped[typ], IndexEntry{Title: e.Title, Link: e.Link, Description: e.Description})
	}

	types := make([]string, 0, len(grouped))
	for t := range grouped {
		types = append(types, t)
	}
	sort.Strings(types)

	sections := make([]IndexSection, 0, len(types))
	for _, t := range types {
		es := grouped[t]
		sort.SliceStable(es, func(i, j int) bool {
			return strings.ToLower(es[i].Title) < strings.ToLower(es[j].Title)
		})
		sections = append(sections, IndexSection{Heading: t, Entries: es})
	}
	return sections
}

// IndexChildSummary is the (title, description) pair passed to a
// Synthesizer describing one child (concept or subdirectory) of a
// directory being indexed.
type IndexChildSummary struct {
	Title       string
	Description string
}

// Synthesizer produces a directory's own summary description from its
// children, for use as that directory's entry in its parent's index. It is
// only invoked when a directory has more than one child, or one child with
// no description of its own (go-okf runs no LLM and no code: this is
// entirely caller-supplied).
type Synthesizer func(dir string, children []IndexChildSummary) string

// RegenerateIndexes synthesizes index.md content for every bundle
// directory that contains at least one concept (directly or via a
// subdirectory), mirroring the reference implementation: entries grouped
// by `type` and sorted by title within a directory's own index;
// subdirectories are listed under a "Subdirectories" heading, reusing a
// lone child's own description or invoking synth for multiple children.
// Directories with no concepts anywhere beneath them are not indexed —
// this is a deliberate simplification of the reference's raw directory
// listing, which will also list a subdirectory containing no concepts at
// all (see NOTES-priorart.md / build report for detail).
//
// Returns bundle-relative directory path ("" for the bundle root) to
// rendered index.md bytes; callers persist.
func (b *Bundle) RegenerateIndexes(synth Synthesizer) map[string][]byte {
	dirSet := map[string]bool{}
	for _, id := range b.sortedIDs() {
		for dir := conceptDir(id); ; dir = conceptDir(dir) {
			dirSet[dir] = true
			if dir == "" {
				break
			}
		}
	}
	if len(dirSet) == 0 {
		return map[string][]byte{}
	}

	dirs := make([]string, 0, len(dirSet))
	for d := range dirSet {
		dirs = append(dirs, d)
	}
	sort.Slice(dirs, func(i, j int) bool {
		di, dj := dirDepth(dirs[i]), dirDepth(dirs[j])
		if di != dj {
			return di > dj
		}
		return dirs[i] < dirs[j]
	})

	childrenByDir := map[string][]string{}
	for _, id := range b.sortedIDs() {
		dir := conceptDir(id)
		childrenByDir[dir] = append(childrenByDir[dir], id)
	}
	subdirsByDir := map[string][]string{}
	for d := range dirSet {
		if d == "" {
			continue
		}
		p := conceptDir(d)
		subdirsByDir[p] = append(subdirsByDir[p], conceptBase(d))
	}

	written := map[string][]byte{}
	descriptions := map[string]string{}

	for _, dir := range dirs {
		var buildEntries []IndexBuildEntry
		var pairs []IndexChildSummary

		for _, id := range childrenByDir[dir] {
			c := b.concepts[ConceptID(id)]
			title := c.Title
			if title == "" {
				title = conceptBase(id)
			}
			buildEntries = append(buildEntries, IndexBuildEntry{
				Type: c.Type, Title: title, Link: conceptBase(id) + ".md", Description: c.Description,
			})
			pairs = append(pairs, IndexChildSummary{Title: title, Description: c.Description})
		}

		subNames := append([]string{}, subdirsByDir[dir]...)
		sort.Strings(subNames)
		for _, name := range subNames {
			childDir := joinDir(dir, name)
			desc := descriptions[childDir]
			buildEntries = append(buildEntries, IndexBuildEntry{
				Type: "Subdirectories", Title: name, Link: name + "/index.md", Description: desc,
			})
			pairs = append(pairs, IndexChildSummary{Title: name, Description: desc})
		}

		if len(buildEntries) == 0 {
			continue
		}

		// Subdirectory entries are fed through the same group/sort path as
		// type groups (group key "Subdirectories") so the heading sorts
		// alphabetically among type headings, matching the reference
		// synthesizer (index.py: `for typ in sorted(grouped)`) rather than
		// always trailing last.
		sections := BuildIndexSections(buildEntries)
		written[dir] = (&Index{Sections: sections}).Bytes()

		if dir == "" {
			continue
		}
		if len(pairs) == 1 && pairs[0].Description != "" {
			descriptions[dir] = pairs[0].Description
		} else if synth != nil {
			descriptions[dir] = synth(dir, pairs)
		}
	}
	return written
}

func conceptDir(id string) string {
	i := strings.LastIndex(id, "/")
	if i < 0 {
		return ""
	}
	return id[:i]
}

func conceptBase(id string) string {
	i := strings.LastIndex(id, "/")
	if i < 0 {
		return id
	}
	return id[i+1:]
}

func dirDepth(dir string) int {
	if dir == "" {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

func joinDir(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}
