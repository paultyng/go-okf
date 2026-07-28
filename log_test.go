package okf

import "testing"

const sampleLog = `# Directory Update Log

## 2026-05-22
* **Update**: Added a BigQuery table reference for [Customer Metrics](/tables/customer-metrics.md).
* **Creation**: Established the [Dataplex Playbook](/playbooks/dataplex.md).

## 2026-05-15
* **Initialization**: Created foundational directory structure.
`

func TestLogParse(t *testing.T) {
	lg, err := ParseLog([]byte(sampleLog))
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	if lg.Heading != "Directory Update Log" {
		t.Errorf("Heading = %q", lg.Heading)
	}
	if len(lg.Sections) != 2 {
		t.Fatalf("len(Sections) = %d: %+v", len(lg.Sections), lg.Sections)
	}
	if lg.Sections[0].Date.Format("2006-01-02") != "2026-05-22" {
		t.Errorf("Sections[0].Date = %v", lg.Sections[0].Date)
	}
	if len(lg.Sections[0].Entries) != 2 {
		t.Fatalf("Sections[0].Entries = %+v", lg.Sections[0].Entries)
	}
	e := lg.Sections[0].Entries[0]
	if e.Kind != "Update" {
		t.Errorf("Kind = %q", e.Kind)
	}
	if e.Text != "Added a BigQuery table reference for [Customer Metrics](/tables/customer-metrics.md)." {
		t.Errorf("Text = %q", e.Text)
	}
	if len(e.Links) != 1 || e.Links[0].Target != "/tables/customer-metrics.md" {
		t.Errorf("Links = %+v", e.Links)
	}

	if lg.Sections[1].Entries[0].Kind != "Initialization" {
		t.Errorf("Sections[1].Entries[0].Kind = %q", lg.Sections[1].Entries[0].Kind)
	}
}

func TestLogBytesRoundTrip(t *testing.T) {
	lg, err := ParseLog([]byte(sampleLog))
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	reparsed, err := ParseLog(lg.Bytes())
	if err != nil {
		t.Fatalf("re-ParseLog: %v", err)
	}
	if reparsed.Heading != lg.Heading || len(reparsed.Sections) != len(lg.Sections) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", reparsed, lg)
	}
	for i := range lg.Sections {
		if len(reparsed.Sections[i].Entries) != len(lg.Sections[i].Entries) {
			t.Errorf("section %d entry count mismatch: %+v vs %+v", i, reparsed.Sections[i], lg.Sections[i])
		}
	}
}

func TestLogInsertNewDateNewestFirst(t *testing.T) {
	lg, err := ParseLog([]byte(sampleLog))
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}

	newest, _ := ParseDate("2026-05-25")
	lg.Insert(LogEntry{Date: newest, Kind: "Update", Text: "Something newer happened."})

	if len(lg.Sections) != 3 {
		t.Fatalf("len(Sections) = %d, want 3", len(lg.Sections))
	}
	if lg.Sections[0].Date.Format("2006-01-02") != "2026-05-25" {
		t.Errorf("newest date should sort first, got %v", lg.Sections[0].Date)
	}
}

func TestLogInsertSameDatePrepends(t *testing.T) {
	lg, err := ParseLog([]byte(sampleLog))
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}

	same, _ := ParseDate("2026-05-22")
	lg.Insert(LogEntry{Date: same, Kind: "Update", Text: "A brand new same-day update."})

	if len(lg.Sections) != 2 {
		t.Fatalf("len(Sections) = %d, want 2 (no new date section)", len(lg.Sections))
	}
	if lg.Sections[0].Entries[0].Text != "A brand new same-day update." {
		t.Errorf("expected the new entry to be prepended, got %+v", lg.Sections[0].Entries[0])
	}
	if len(lg.Sections[0].Entries) != 3 {
		t.Errorf("expected 3 entries in the 2026-05-22 section, got %d", len(lg.Sections[0].Entries))
	}
}

func TestLogInsertOldestDateAppendsSection(t *testing.T) {
	lg, err := ParseLog([]byte(sampleLog))
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	older, _ := ParseDate("2026-01-01")
	lg.Insert(LogEntry{Date: older, Kind: "Initialization", Text: "Even older."})
	if len(lg.Sections) != 3 {
		t.Fatalf("len(Sections) = %d, want 3", len(lg.Sections))
	}
	if lg.Sections[2].Date.Format("2006-01-02") != "2026-01-01" {
		t.Errorf("oldest date should sort last, got %v", lg.Sections[2].Date)
	}
}
