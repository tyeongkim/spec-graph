package bootstrap

import (
	"encoding/json"
	"fmt"
	"os"
)

// ReviewResult holds candidates formatted for review output (no DB interaction).
type ReviewResult struct {
	Entities  []EntityCandidate   `json:"entities"`
	Relations []RelationCandidate `json:"relations"`
}

// ReviewCandidates returns the scan result as-is for human review.
func ReviewCandidates(input ScanResult) ReviewResult {
	return ReviewResult{
		Entities:  input.Entities,
		Relations: input.Relations,
	}
}

// LoadCandidatesFromFile reads a JSON file previously written by scan --output.
func LoadCandidatesFromFile(path string) (ScanResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ScanResult{}, fmt.Errorf("read candidates file: %w", err)
	}
	var result ScanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ScanResult{}, fmt.Errorf("parse candidates JSON: %w", err)
	}
	return result, nil
}
