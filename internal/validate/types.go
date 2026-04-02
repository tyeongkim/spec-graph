// Package validate provides layered validation for spec-graph entities and relations.
package validate

import "github.com/tyeongkim/spec-graph/internal/model"

// Severity represents the severity level of a validation issue.
type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
)

// RelationFetcher retrieves relations for a given entity.
type RelationFetcher interface {
	GetByEntity(entityID string) ([]model.Relation, error)
}

// EntityListFilters filters entities by type, status, and/or layer.
type EntityListFilters struct {
	Type   *model.EntityType
	Status *model.EntityStatus
	Layer  *model.Layer
}

// EntityFetcher retrieves entities by ID or filtered list.
type EntityFetcher interface {
	Get(id string) (model.Entity, error)
	List(filters EntityListFilters) ([]model.Entity, error)
}

// ValidationIssue represents a single problem found during validation.
type ValidationIssue struct {
	Check    string      `json:"check"`
	Severity Severity    `json:"severity"`
	Entity   string      `json:"entity"`
	Message  string      `json:"message"`
	Layer    model.Layer `json:"layer"`
}

// ValidateOptions controls which checks are run during validation.
type ValidateOptions struct {
	// Checks lists the check names to run. nil = all checks for the selected layer(s).
	Checks []string
	// Phase restricts validation to entities belonging to this phase. nil = all entities.
	Phase *string
	// EntityID restricts reported issues to those for this specific entity. "" = all entities.
	EntityID string
	// Layer restricts validation to this layer. nil = all layers.
	Layer *model.Layer
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

// Architecture layer check names.
var ArchChecks = []string{
	"orphans",
	"coverage",
	"invalid_edges",
	"superseded_refs",
	"unresolved",
	"cycles",
	"conflicts",
}

// Execution layer check names.
var ExecChecks = []string{
	"phase_order",
	"single_active_plan",
	"orphan_phases",
	"exec_cycles",
	"invalid_exec_edges",
}

// Mapping layer check names.
var MappingChecks = []string{
	"plan_coverage",
	"delivery_completeness",
	"mapping_consistency",
	"invalid_mapping_edges",
	"gates",
}

// allChecks is the union of all layer checks.
var allChecks = func() map[string]model.Layer {
	m := make(map[string]model.Layer)
	for _, c := range ArchChecks {
		m[c] = model.LayerArch
	}
	for _, c := range ExecChecks {
		m[c] = model.LayerExec
	}
	for _, c := range MappingChecks {
		m[c] = model.LayerMapping
	}
	return m
}()

// CheckLayer returns the layer a check belongs to and whether it is known.
func CheckLayer(check string) (model.Layer, bool) {
	l, ok := allChecks[check]
	return l, ok
}
