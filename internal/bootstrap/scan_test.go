package bootstrap

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestScanFile_EntityExtraction(t *testing.T) {
	testdata := filepath.Join("testdata", "requirements.md")

	result, err := ScanFile(testdata)
	if err != nil {
		t.Fatalf("ScanFile(%q) error: %v", testdata, err)
	}

	entityByID := make(map[string]EntityCandidate)
	for _, e := range result.Entities {
		entityByID[e.ID] = e
	}

	tests := []struct {
		name      string
		id        string
		wantType  string
		wantTitle string
		minConf   float64
		maxConf   float64
	}{
		{
			name:      "heading entity with title",
			id:        "REQ-001",
			wantType:  "requirement",
			wantTitle: "All APIs require authentication",
			minConf:   0.9,
			maxConf:   0.9,
		},
		{
			name:      "heading entity with title 2",
			id:        "REQ-002",
			wantType:  "requirement",
			wantTitle: "Rate limiting on public endpoints",
			minConf:   0.9,
			maxConf:   0.9,
		},
		{
			name:     "inline mention without heading",
			id:       "REQ-003",
			wantType: "requirement",
			minConf:  0.5,
			maxConf:  0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ok := entityByID[tt.id]
			if !ok {
				t.Fatalf("entity %s not found", tt.id)
			}
			if e.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", e.Type, tt.wantType)
			}
			if tt.wantTitle != "" && e.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", e.Title, tt.wantTitle)
			}
			if e.Confidence < tt.minConf || e.Confidence > tt.maxConf {
				t.Errorf("Confidence = %v, want [%v, %v]", e.Confidence, tt.minConf, tt.maxConf)
			}
		})
	}

	if len(result.Entities) != 3 {
		t.Errorf("got %d entities, want 3", len(result.Entities))
	}
}

func TestScanFile_RelationExtraction(t *testing.T) {
	testdata := filepath.Join("testdata", "decisions.md")

	result, err := ScanFile(testdata)
	if err != nil {
		t.Fatalf("ScanFile(%q) error: %v", testdata, err)
	}

	type relKey struct {
		from, to, typ string
	}
	relMap := make(map[relKey]RelationCandidate)
	for _, r := range result.Relations {
		relMap[relKey{r.From, r.To, r.Type}] = r
	}

	tests := []struct {
		name     string
		from     string
		to       string
		relType  string
		wantConf float64
	}{
		{
			name:     "implements relation",
			from:     "API-005",
			to:       "REQ-001",
			relType:  "implements",
			wantConf: 0.8,
		},
		{
			name:     "verifies relation",
			from:     "TST-010",
			to:       "REQ-002",
			relType:  "verifies",
			wantConf: 0.8,
		},
		{
			name:     "depends_on relation",
			from:     "DEC-001",
			to:       "REQ-001",
			relType:  "depends_on",
			wantConf: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := relKey{tt.from, tt.to, tt.relType}
			r, ok := relMap[key]
			if !ok {
				t.Fatalf("relation %s -[%s]-> %s not found; have %d relations total",
					tt.from, tt.relType, tt.to, len(result.Relations))
			}
			if r.Confidence != tt.wantConf {
				t.Errorf("Confidence = %v, want %v", r.Confidence, tt.wantConf)
			}
			if r.Source == "" {
				t.Error("Source should not be empty")
			}
		})
	}
}

func TestScanFile_SourceFormat(t *testing.T) {
	testdata := filepath.Join("testdata", "requirements.md")

	result, err := ScanFile(testdata)
	if err != nil {
		t.Fatalf("ScanFile(%q) error: %v", testdata, err)
	}

	for _, e := range result.Entities {
		if e.ID == "REQ-001" {
			want := "requirements.md#L1"
			if e.Source != want {
				t.Errorf("Source = %q, want %q", e.Source, want)
			}
			return
		}
	}
	t.Fatal("REQ-001 not found")
}

func TestScanFile_TypeInference(t *testing.T) {
	tests := []struct {
		prefix   string
		wantType string
	}{
		{"REQ", "requirement"},
		{"DEC", "decision"},
		{"PHS", "phase"},
		{"API", "interface"},
		{"STT", "state"},
		{"TST", "test"},
		{"XCT", "crosscut"},
		{"QST", "question"},
		{"ASM", "assumption"},
		{"ACT", "criterion"},
		{"RSK", "risk"},
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			got := inferType(tt.prefix + "-001")
			if got != tt.wantType {
				t.Errorf("inferType(%s-001) = %q, want %q", tt.prefix, got, tt.wantType)
			}
		})
	}
}

func TestScanFile_NonExistentFile(t *testing.T) {
	_, err := ScanFile("testdata/nonexistent.md")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestScanDirectory_IncludesMdExcludesTxt(t *testing.T) {
	result, err := ScanDirectory("testdata")
	if err != nil {
		t.Fatalf("ScanDirectory error: %v", err)
	}

	entityIDs := make(map[string]bool)
	for _, e := range result.Entities {
		entityIDs[e.ID] = true
	}

	if entityIDs["REQ-999"] {
		t.Error("REQ-999 from notes.txt should not be found")
	}

	if !entityIDs["REQ-001"] {
		t.Error("REQ-001 from requirements.md should be found")
	}
	if !entityIDs["DEC-001"] {
		t.Error("DEC-001 from decisions.md should be found")
	}
}

func TestScanDirectory_Deduplication(t *testing.T) {
	result, err := ScanDirectory("testdata")
	if err != nil {
		t.Fatalf("ScanDirectory error: %v", err)
	}

	idCounts := make(map[string]int)
	for _, e := range result.Entities {
		idCounts[e.ID]++
	}

	for id, count := range idCounts {
		if count > 1 {
			t.Errorf("entity %s appears %d times, want 1 (dedup failed)", id, count)
		}
	}

	for _, e := range result.Entities {
		if e.ID == "REQ-001" {
			if e.Confidence < 0.9 {
				t.Errorf("REQ-001 confidence = %v, want >= 0.9 (heading version should win)", e.Confidence)
			}
			return
		}
	}
	t.Fatal("REQ-001 not found after dedup")
}

func TestScanDirectory_NonExistent(t *testing.T) {
	_, err := ScanDirectory("testdata/nonexistent_dir")
	if err == nil {
		t.Fatal("expected error for non-existent directory")
	}
}

func TestScanDirectory_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := ScanDirectory(tmpDir)
	if err != nil {
		t.Fatalf("ScanDirectory error: %v", err)
	}

	if len(result.Entities) != 0 {
		t.Errorf("got %d entities, want 0", len(result.Entities))
	}
	if len(result.Relations) != 0 {
		t.Errorf("got %d relations, want 0", len(result.Relations))
	}
}

func TestScanFile_ProximityRelation(t *testing.T) {
	tmpDir := t.TempDir()
	content := "REQ-001 and REQ-002 are related somehow\n"
	path := filepath.Join(tmpDir, "proximity.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ScanFile(path)
	if err != nil {
		t.Fatalf("ScanFile error: %v", err)
	}

	var found bool
	for _, r := range result.Relations {
		if r.From == "REQ-001" && r.To == "REQ-002" && r.Confidence == 0.4 {
			found = true
			if r.Type != "references" {
				t.Errorf("proximity relation Type = %q, want %q", r.Type, "references")
			}
			break
		}
	}
	if !found {
		t.Error("expected proximity relation between REQ-001 and REQ-002 with confidence 0.4")
	}
}

func TestScanFile_ConfidenceScoring(t *testing.T) {
	tmpDir := t.TempDir()
	content := `# REQ-100 Important requirement
Some text mentioning REQ-200 inline.
`
	path := filepath.Join(tmpDir, "confidence.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ScanFile(path)
	if err != nil {
		t.Fatalf("ScanFile error: %v", err)
	}

	entityByID := make(map[string]EntityCandidate)
	for _, e := range result.Entities {
		entityByID[e.ID] = e
	}

	heading := entityByID["REQ-100"]
	inline := entityByID["REQ-200"]

	if heading.Confidence <= inline.Confidence {
		t.Errorf("heading confidence (%v) should be > inline confidence (%v)",
			heading.Confidence, inline.Confidence)
	}
}

func TestScanFile_HeadingWithColon(t *testing.T) {
	tmpDir := t.TempDir()
	content := `## REQ-100: User Authentication
All users must authenticate.

## DEC-200: Use JWT Tokens
We decided to use JWT.
`
	path := filepath.Join(tmpDir, "colon.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ScanFile(path)
	if err != nil {
		t.Fatalf("ScanFile error: %v", err)
	}

	entityByID := make(map[string]EntityCandidate)
	for _, e := range result.Entities {
		entityByID[e.ID] = e
	}

	tests := []struct {
		id        string
		wantTitle string
		wantConf  float64
	}{
		{"REQ-100", "User Authentication", 0.9},
		{"DEC-200", "Use JWT Tokens", 0.9},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			e, ok := entityByID[tt.id]
			if !ok {
				t.Fatalf("entity %s not found", tt.id)
			}
			if e.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", e.Title, tt.wantTitle)
			}
			if e.Confidence != tt.wantConf {
				t.Errorf("Confidence = %v, want %v", e.Confidence, tt.wantConf)
			}
		})
	}
}

func TestScanDirectory_AllEntityTypes(t *testing.T) {
	tmpDir := t.TempDir()
	content := `# REQ-001 Requirement
# DEC-001 Decision
# PHS-001 Phase
# API-001 Interface
# STT-001 State
# TST-001 Test
# XCT-001 Crosscut
# QST-001 Question
# ASM-001 Assumption
# ACT-001 Criterion
# RSK-001 Risk
`
	if err := os.WriteFile(filepath.Join(tmpDir, "all.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ScanDirectory(tmpDir)
	if err != nil {
		t.Fatalf("ScanDirectory error: %v", err)
	}

	ids := make([]string, 0, len(result.Entities))
	for _, e := range result.Entities {
		ids = append(ids, e.ID)
	}
	sort.Strings(ids)

	want := []string{"ACT-001", "API-001", "ASM-001", "DEC-001", "PHS-001", "QST-001", "REQ-001", "RSK-001", "STT-001", "TST-001", "XCT-001"}
	sort.Strings(want)

	if len(ids) != len(want) {
		t.Fatalf("got %d entities %v, want %d %v", len(ids), ids, len(want), want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("entity[%d] = %q, want %q", i, ids[i], want[i])
		}
	}
}
