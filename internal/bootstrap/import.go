package bootstrap

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/taeyeong/spec-graph/internal/model"
	"github.com/taeyeong/spec-graph/internal/store"
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

// ApplyResult summarises the outcome of applying candidates to the DB.
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

// ApplyCandidates imports candidates into the DB via the normal store layer.
// Low-confidence candidates (<0.5) are skipped. Duplicates and invalid edges
// are recorded as skipped items rather than hard errors.
func ApplyCandidates(input ScanResult, es *store.EntityStore, rs *store.RelationStore) ApplyResult {
	var result ApplyResult

	for _, c := range input.Entities {
		if c.Confidence < 0.5 {
			result.Skipped = append(result.Skipped, SkippedItem{
				ID: c.ID, Reason: "low confidence",
			})
			continue
		}

		entity := model.Entity{
			ID:    c.ID,
			Type:  model.EntityType(c.Type),
			Layer: model.Layer(c.Layer),
			Title: c.Title,
		}
		if _, err := es.Create(entity, "bootstrap import", "", "bootstrap"); err != nil {
			if isDuplicateEntityError(err) {
				result.Skipped = append(result.Skipped, SkippedItem{
					ID: c.ID, Reason: "already exists",
				})
			} else {
				result.Errors = append(result.Errors, ErrorItem{
					ID: c.ID, Error: err.Error(),
				})
			}
			continue
		}
		result.Created = append(result.Created, c.ID)
	}

	for _, c := range input.Relations {
		key := fmt.Sprintf("%s:%s:%s", c.From, c.To, c.Type)

		if c.Confidence < 0.5 {
			result.Skipped = append(result.Skipped, SkippedItem{
				ID: key, Reason: "low confidence",
			})
			continue
		}

		rel := model.Relation{
			FromID: c.From,
			ToID:   c.To,
			Type:   model.RelationType(c.Type),
		}
		if _, err := rs.Create(rel, "bootstrap import", "", "bootstrap"); err != nil {
			if isInvalidEdgeError(err) {
				result.Skipped = append(result.Skipped, SkippedItem{
					ID: key, Reason: "invalid edge",
				})
			} else if isDuplicateRelationError(err) {
				result.Skipped = append(result.Skipped, SkippedItem{
					ID: key, Reason: "already exists",
				})
			} else {
				result.Errors = append(result.Errors, ErrorItem{
					ID: key, Error: err.Error(),
				})
			}
			continue
		}
		result.Created = append(result.Created, key)
	}

	return result
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

func isDuplicateEntityError(err error) bool {
	var target *model.ErrDuplicateEntity
	return errors.As(err, &target)
}

func isInvalidEdgeError(err error) bool {
	var target *model.ErrInvalidEdge
	return errors.As(err, &target)
}

func isDuplicateRelationError(err error) bool {
	var target *model.ErrDuplicateRelation
	return errors.As(err, &target)
}
