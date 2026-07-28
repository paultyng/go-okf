package okf

import (
	"io/fs"
	"sort"
	"strings"
)

// Bundle is an in-memory OKF knowledge bundle: concepts keyed by concept
// ID, plus any reserved files (index.md, log.md) found at each directory
// level.
type Bundle struct {
	concepts map[ConceptID]*Concept
	indexes  map[string]*Index
	logs     map[string]*Log
}

// Load reads a bundle from any [fs.FS] (os.DirFS, embed.FS, fstest.MapFS,
// ...). It is permissive per OKF v0.2 §11: a concept file with unparsable
// frontmatter is skipped rather than failing the whole load, unknown types
// are accepted, and broken links are never checked here (link resolution
// is a consumer/extraction concern, not a load-time one).
func Load(fsys fs.FS) (*Bundle, error) {
	b := &Bundle{
		concepts: map[ConceptID]*Concept{},
		indexes:  map[string]*Index{},
		logs:     map[string]*Log{},
	}

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}

		dir := dirOf(path)
		base := baseName(path)
		switch base {
		case "index.md":
			if idx, err := ParseIndex(data); err == nil {
				b.indexes[dir] = idx
			}
		case "log.md":
			if lg, err := ParseLog(data); err == nil {
				b.logs[dir] = lg
			}
		default:
			if c, err := Parse(data); err == nil {
				b.concepts[ConceptIDFromPath(path)] = c
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return b, nil
}

// FromConcepts builds a [Bundle] directly from an in-memory concept map,
// keyed by concept id string, without any filesystem involved.
func FromConcepts(concepts map[string]*Concept) *Bundle {
	b := &Bundle{concepts: map[ConceptID]*Concept{}, indexes: map[string]*Index{}, logs: map[string]*Log{}}
	for id, c := range concepts {
		b.concepts[ConceptID(id)] = c
	}
	return b
}

// Concept looks up a concept by id.
func (b *Bundle) Concept(id string) (*Concept, bool) {
	c, ok := b.concepts[ConceptID(id)]
	return c, ok
}

// sortedIDs returns every concept id in the bundle, sorted, for
// deterministic iteration across all Bundle aggregation methods.
func (b *Bundle) sortedIDs() []string {
	ids := make([]string, 0, len(b.concepts))
	for id := range b.concepts {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	return ids
}

// Concepts returns every concept in the bundle, ordered by concept id.
func (b *Bundle) Concepts() []*Concept {
	ids := b.sortedIDs()
	out := make([]*Concept, 0, len(ids))
	for _, id := range ids {
		out = append(out, b.concepts[ConceptID(id)])
	}
	return out
}

// Index returns the parsed index.md at the given bundle-relative directory
// ("" for the bundle root), if one was loaded.
func (b *Bundle) Index(dir string) (*Index, bool) {
	idx, ok := b.indexes[dir]
	return idx, ok
}

// Log returns the parsed log.md at the given bundle-relative directory
// ("" for the bundle root), if one was loaded.
func (b *Bundle) Log(dir string) (*Log, bool) {
	lg, ok := b.logs[dir]
	return lg, ok
}

func dirOf(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return ""
	}
	return path[:i]
}

func baseName(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return path
	}
	return path[i+1:]
}
