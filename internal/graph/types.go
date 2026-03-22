// Package graph provides the core graph traversal and analysis logic
// for impact analysis and validation of spec-graph entities.
package graph

import "github.com/taeyeong/spec-graph/internal/model"

// RelationFetcher retrieves relations for a given entity.
type RelationFetcher interface {
	GetByEntity(entityID string) ([]model.Relation, error)
}

// EntityListFilters filters entities by type and/or status.
type EntityListFilters struct {
	Type   *model.EntityType
	Status *model.EntityStatus
}

// EntityFetcher retrieves entities by ID or filtered list.
type EntityFetcher interface {
	Get(id string) (model.Entity, error)
	List(filters EntityListFilters) ([]model.Entity, error)
}

// ImpactOptions controls how impact traversal is performed.
type ImpactOptions struct {
	// Follow restricts traversal to these relation types. nil = all types.
	Follow []model.RelationType
	// MinSeverity filters out affected entities below this severity. nil = no filter.
	MinSeverity *Severity
	// Dimension restricts scoring to a single dimension. nil = all dimensions.
	Dimension *string
}

// AffectedEntity describes a single entity reached during impact traversal.
type AffectedEntity struct {
	ID            string               `json:"id"`
	Type          model.EntityType     `json:"type"`
	Depth         int                  `json:"depth"`
	Path          []string             `json:"path"`
	RelationChain []model.RelationType `json:"relation_chain"`
	Impact        DimensionScores      `json:"impact"`
	Overall       Severity             `json:"overall"`
	Reason        string               `json:"reason"`
}

// ImpactSummary aggregates counts from an impact traversal.
type ImpactSummary struct {
	Total    int                      `json:"total"`
	ByType   map[model.EntityType]int `json:"by_type"`
	ByImpact map[Severity]int         `json:"by_impact"`
}

// ImpactResult is the top-level result returned by impact analysis.
type ImpactResult struct {
	Sources  []string         `json:"sources"`
	Affected []AffectedEntity `json:"affected"`
	Summary  ImpactSummary    `json:"summary"`
}

// ValidateOptions controls which checks are run during validation.
type ValidateOptions struct {
	// Checks lists the check names to run. nil = all checks.
	Checks []string
	// Phase restricts validation to entities belonging to this phase. nil = all entities.
	Phase *string
}

// ValidationIssue represents a single problem found during validation.
type ValidationIssue struct {
	Check    string   `json:"check"`
	Severity Severity `json:"severity"`
	Entity   string   `json:"entity"`
	Message  string   `json:"message"`
}

// ValidateSummary aggregates counts from a validation run.
type ValidateSummary struct {
	TotalIssues int              `json:"total_issues"`
	BySeverity  map[Severity]int `json:"by_severity"`
}

// ValidateResult is the top-level result returned by validation.
type ValidateResult struct {
	Valid   bool              `json:"valid"`
	Issues  []ValidationIssue `json:"issues"`
	Summary ValidateSummary   `json:"summary"`
}
