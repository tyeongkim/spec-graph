// Package bootstrap provides pattern-based extraction of entities and relations
// from markdown files, producing candidates with confidence scores.
package bootstrap

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// EntityCandidate represents a potential entity found in markdown.
type EntityCandidate struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Title      string  `json:"title"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"`
}

// RelationCandidate represents a potential relation found in markdown.
type RelationCandidate struct {
	From       string  `json:"from"`
	To         string  `json:"to"`
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"`
}

// ScanResult holds all candidates extracted from scanning.
type ScanResult struct {
	Entities  []EntityCandidate   `json:"entities"`
	Relations []RelationCandidate `json:"relations"`
}

// prefixTypeMap maps entity ID prefixes to their semantic type names.
var prefixTypeMap = map[string]string{
	"REQ": "requirement",
	"DEC": "decision",
	"PHS": "phase",
	"API": "interface",
	"STT": "state",
	"TST": "test",
	"XCT": "crosscut",
	"QST": "question",
	"ASM": "assumption",
	"ACT": "criterion",
	"RSK": "risk",
}

// relationKeywords maps human-readable keywords to canonical relation type names.
var relationKeywords = map[string]string{
	"implements":     "implements",
	"verifies":       "verifies",
	"depends on":     "depends_on",
	"depends_on":     "depends_on",
	"constrained by": "constrained_by",
	"constrained_by": "constrained_by",
	"planned in":     "planned_in",
	"planned_in":     "planned_in",
	"delivered in":   "delivered_in",
	"delivered_in":   "delivered_in",
	"triggers":       "triggers",
	"answers":        "answers",
	"assumes":        "assumes",
	"has criterion":  "has_criterion",
	"has_criterion":  "has_criterion",
	"mitigates":      "mitigates",
	"supersedes":     "supersedes",
	"conflicts with": "conflicts_with",
	"conflicts_with": "conflicts_with",
	"references":     "references",
}

var (
	// entityIDRe matches entity IDs like REQ-001, DEC-005, etc.
	entityIDRe = regexp.MustCompile(`(REQ|DEC|PHS|API|STT|TST|XCT|QST|ASM|ACT|RSK)-\d+`)

	// headingEntityRe matches entity IDs at the start of a heading line, capturing the title.
	// Supports optional colon after ID: "## REQ-001: Title" or "## REQ-001 Title".
	headingEntityRe = regexp.MustCompile(`^#+\s+((?:REQ|DEC|PHS|API|STT|TST|XCT|QST|ASM|ACT|RSK)-\d+)[:\s]\s*(.*)$`)

	// lineStartEntityRe matches entity IDs at the very start of a line (no heading marker).
	lineStartEntityRe = regexp.MustCompile(`^((?:REQ|DEC|PHS|API|STT|TST|XCT|QST|ASM|ACT|RSK)-\d+)\s+(.*)$`)
)

// buildRelationRe constructs a compiled regex for a given keyword that captures
// <ID> <keyword> <ID> patterns (case-insensitive).
func buildRelationRe(keyword string) *regexp.Regexp {
	escaped := regexp.QuoteMeta(keyword)
	pattern := fmt.Sprintf(
		`(?i)((?:REQ|DEC|PHS|API|STT|TST|XCT|QST|ASM|ACT|RSK)-\d+)\s+%s\s+((?:REQ|DEC|PHS|API|STT|TST|XCT|QST|ASM|ACT|RSK)-\d+)`,
		escaped,
	)
	return regexp.MustCompile(pattern)
}

// precompiledRelationRes holds compiled regexes for each relation keyword.
var precompiledRelationRes map[string]*regexp.Regexp

func init() {
	precompiledRelationRes = make(map[string]*regexp.Regexp, len(relationKeywords))
	for kw := range relationKeywords {
		precompiledRelationRes[kw] = buildRelationRe(kw)
	}
}

// inferType returns the entity type string for a given entity ID prefix.
func inferType(id string) string {
	parts := strings.SplitN(id, "-", 2)
	if len(parts) < 2 {
		return ""
	}
	return prefixTypeMap[parts[0]]
}

// sourceRef formats a source reference as "filename#L<line>".
func sourceRef(filePath string, lineNum int) string {
	return fmt.Sprintf("%s#L%d", filepath.Base(filePath), lineNum)
}

// ScanFile scans a single markdown file and returns extracted entity and relation candidates.
func ScanFile(filePath string) (ScanResult, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return ScanResult{}, fmt.Errorf("open %s: %w", filePath, err)
	}
	defer f.Close()

	var result ScanResult
	entityMap := make(map[string]EntityCandidate)

	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		src := sourceRef(filePath, lineNum)

		extractEntities(line, src, entityMap)
		extractRelations(line, src, &result)
	}

	if err := scanner.Err(); err != nil {
		return ScanResult{}, fmt.Errorf("scan %s: %w", filePath, err)
	}

	result.Entities = make([]EntityCandidate, 0, len(entityMap))
	for _, e := range entityMap {
		result.Entities = append(result.Entities, e)
	}

	return result, nil
}

// extractEntities finds entity IDs in a line and updates the entityMap,
// keeping the highest-confidence occurrence.
func extractEntities(line, src string, entityMap map[string]EntityCandidate) {
	if m := headingEntityRe.FindStringSubmatch(line); m != nil {
		id := m[1]
		title := strings.TrimSpace(m[2])
		conf := 0.8
		if title != "" {
			conf = 0.9
		}
		candidate := EntityCandidate{
			ID:         id,
			Type:       inferType(id),
			Title:      title,
			Confidence: conf,
			Source:     src,
		}
		if existing, ok := entityMap[id]; !ok || candidate.Confidence > existing.Confidence {
			entityMap[id] = candidate
		}
		return
	}

	if m := lineStartEntityRe.FindStringSubmatch(line); m != nil {
		id := m[1]
		title := strings.TrimSpace(m[2])
		conf := 0.5
		candidate := EntityCandidate{
			ID:         id,
			Type:       inferType(id),
			Title:      title,
			Confidence: conf,
			Source:     src,
		}
		if existing, ok := entityMap[id]; !ok || candidate.Confidence > existing.Confidence {
			entityMap[id] = candidate
		}
	}

	matches := entityIDRe.FindAllString(line, -1)
	for _, id := range matches {
		if _, ok := entityMap[id]; ok {
			continue
		}
		entityMap[id] = EntityCandidate{
			ID:         id,
			Type:       inferType(id),
			Confidence: 0.5,
			Source:     src,
		}
	}
}

// extractRelations finds relation patterns in a line and appends them to result.
func extractRelations(line, src string, result *ScanResult) {
	matched := make(map[string]bool)

	for kw, re := range precompiledRelationRes {
		ms := re.FindAllStringSubmatch(line, -1)
		for _, m := range ms {
			from := strings.ToUpper(m[1])
			to := strings.ToUpper(m[2])
			key := from + "->" + to
			if matched[key] {
				continue
			}
			matched[key] = true
			result.Relations = append(result.Relations, RelationCandidate{
				From:       from,
				To:         to,
				Type:       relationKeywords[kw],
				Confidence: 0.8,
				Source:     src,
			})
		}
	}

	// Proximity: two IDs on same line without explicit keyword.
	ids := entityIDRe.FindAllString(line, -1)
	if len(ids) >= 2 {
		for i := 0; i < len(ids)-1; i++ {
			for j := i + 1; j < len(ids); j++ {
				key := ids[i] + "->" + ids[j]
				if matched[key] {
					continue
				}
				result.Relations = append(result.Relations, RelationCandidate{
					From:       ids[i],
					To:         ids[j],
					Type:       "references",
					Confidence: 0.4,
					Source:     src,
				})
			}
		}
	}
}

// ScanDirectory scans all .md files in the given directory (non-recursive)
// and returns a merged, deduplicated ScanResult.
func ScanDirectory(inputPath string) (ScanResult, error) {
	entries, err := os.ReadDir(inputPath)
	if err != nil {
		return ScanResult{}, fmt.Errorf("read directory %s: %w", inputPath, err)
	}

	merged := ScanResult{}
	entityMap := make(map[string]EntityCandidate)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}

		filePath := filepath.Join(inputPath, entry.Name())
		fileResult, err := ScanFile(filePath)
		if err != nil {
			return ScanResult{}, fmt.Errorf("scan file %s: %w", filePath, err)
		}

		for _, e := range fileResult.Entities {
			if existing, ok := entityMap[e.ID]; !ok || e.Confidence > existing.Confidence {
				entityMap[e.ID] = e
			}
		}

		merged.Relations = append(merged.Relations, fileResult.Relations...)
	}

	merged.Entities = make([]EntityCandidate, 0, len(entityMap))
	for _, e := range entityMap {
		merged.Entities = append(merged.Entities, e)
	}

	return merged, nil
}
