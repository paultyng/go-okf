package okf

import (
	"fmt"
	"regexp"
	"strings"
)

// LogEntry is one bullet-list item in a log.md date section (OKF v0.2 §9).
// Kind is the leading bold word convention ("Update", "Creation",
// "Initialization", or any free-form word); it is empty when the entry has
// no leading bold word. Links is derived from Text, not stored.
type LogEntry struct {
	Date  Date
	Kind  string
	Text  string
	Links []Link
}

// LogSection groups a log.md's entries under one date heading.
type LogSection struct {
	Date    Date
	Entries []LogEntry
}

// Log is a parsed (or newly built) log.md: a flat, date-grouped,
// newest-first chronological history of changes (OKF v0.2 §9).
type Log struct {
	Heading  string
	Sections []LogSection
}

const defaultLogHeading = "Directory Update Log"

var (
	logHeading1Re = regexp.MustCompile(`^#\s+(.+?)\s*$`)
	logHeading2Re = regexp.MustCompile(`^##\s+(.+?)\s*$`)
	logItemRe     = regexp.MustCompile(`^[*-]\s+(.*)$`)
	logKindRe     = regexp.MustCompile(`^\*\*([^*]+)\*\*:?\s*(.*)$`)
)

// ParseLog parses a log.md's raw bytes.
func ParseLog(data []byte) (*Log, error) {
	log := &Log{}
	var cur *LogSection

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, " \t\r")
		if line == "" {
			continue
		}
		if m := logHeading2Re.FindStringSubmatch(line); m != nil {
			d, err := ParseDate(strings.TrimSpace(m[1]))
			if err != nil {
				continue
			}
			log.Sections = append(log.Sections, LogSection{Date: d})
			cur = &log.Sections[len(log.Sections)-1]
			continue
		}
		if m := logHeading1Re.FindStringSubmatch(line); m != nil {
			log.Heading = m[1]
			continue
		}
		if m := logItemRe.FindStringSubmatch(line); m != nil {
			if cur == nil {
				continue
			}
			cur.Entries = append(cur.Entries, parseLogItem(m[1]))
		}
	}
	return log, nil
}

func parseLogItem(raw string) LogEntry {
	entry := LogEntry{Text: raw}
	if m := logKindRe.FindStringSubmatch(raw); m != nil {
		entry.Kind = strings.TrimSpace(m[1])
		entry.Text = strings.TrimSpace(m[2])
	}
	doc, source := parseBody(extractMarkdown, entry.Text)
	entry.Links = extractLinks(doc, source)
	return entry
}

// Insert adds entry to the log: finding or creating its date section
// (newest-first order among sections) and prepending the entry within
// that section (newest-first within a day). The date is supplied by the
// caller — this package never calls [time.Now].
func (l *Log) Insert(entry LogEntry) {
	for i := range l.Sections {
		if l.Sections[i].Date.Time.Equal(entry.Date.Time) {
			l.Sections[i].Entries = append([]LogEntry{entry}, l.Sections[i].Entries...)
			return
		}
	}

	idx := 0
	for idx < len(l.Sections) && l.Sections[idx].Date.Time.After(entry.Date.Time) {
		idx++
	}
	l.Sections = append(l.Sections, LogSection{})
	copy(l.Sections[idx+1:], l.Sections[idx:])
	l.Sections[idx] = LogSection{Date: entry.Date, Entries: []LogEntry{entry}}
}

// Bytes renders the log back to markdown.
func (l *Log) Bytes() []byte {
	heading := l.Heading
	if heading == "" {
		heading = defaultLogHeading
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", heading)
	for i, sec := range l.Sections {
		fmt.Fprintf(&b, "## %s\n", sec.Date.Time.Format("2006-01-02"))
		for _, e := range sec.Entries {
			if e.Kind != "" {
				fmt.Fprintf(&b, "* **%s**: %s\n", e.Kind, e.Text)
			} else {
				fmt.Fprintf(&b, "* %s\n", e.Text)
			}
		}
		if i != len(l.Sections)-1 {
			b.WriteString("\n")
		}
	}
	return []byte(b.String())
}
