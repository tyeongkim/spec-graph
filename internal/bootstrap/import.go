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

// SkippedItem records a candidate that was skipped during apply.
type SkippedItem struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// ErrorItem records a candidate that failed during apply.
type ErrorItem struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

// ApplyResult summarises the outcome of an import, as reported by the engine.
type ApplyResult struct {
	Created []string      `json:"created"`
	Skipped []SkippedItem `json:"skipped"`
	Errors  []ErrorItem   `json:"errors"`
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
