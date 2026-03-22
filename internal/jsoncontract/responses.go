package jsoncontract

import "github.com/taeyeong/spec-graph/internal/model"

type EntityResponse struct {
	Entity model.Entity `json:"entity"`
}

type EntityListResponse struct {
	Entities []model.Entity `json:"entities"`
	Count    int            `json:"count"`
}

type RelationResponse struct {
	Relation model.Relation `json:"relation"`
}

type RelationListResponse struct {
	Relations []model.Relation `json:"relations"`
	Count     int              `json:"count"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type DeleteResponse struct {
	Deleted string `json:"deleted"`
}

type InitResponse struct {
	Initialized bool   `json:"initialized"`
	Path        string `json:"path"`
}

// ImpactResponse is the top-level JSON response for the impact command.
type ImpactResponse struct {
	Sources  []string         `json:"sources"`
	Affected []ImpactAffected `json:"affected"`
	Summary  ImpactSummary    `json:"summary"`
}

// ImpactAffected describes a single affected entity in impact output.
type ImpactAffected struct {
	ID            string       `json:"id"`
	Type          string       `json:"type"`
	Depth         int          `json:"depth"`
	Path          []string     `json:"path"`
	RelationChain []string     `json:"relation_chain"`
	Impact        ImpactScores `json:"impact"`
	Reason        string       `json:"reason"`
}

// ImpactScores holds the nested impact scoring object.
type ImpactScores struct {
	Overall    string `json:"overall"`
	Structural string `json:"structural"`
	Behavioral string `json:"behavioral"`
	Planning   string `json:"planning"`
}

// ImpactSummary holds aggregated counts for impact output.
type ImpactSummary struct {
	Total    int            `json:"total"`
	ByType   map[string]int `json:"by_type"`
	ByImpact map[string]int `json:"by_impact"`
}

// ValidateResponse is the top-level JSON response for the validate command.
type ValidateResponse struct {
	Valid   bool            `json:"valid"`
	Issues  []ValidateIssue `json:"issues"`
	Summary ValidateSummary `json:"summary"`
}

// ValidateIssue describes a single validation issue.
type ValidateIssue struct {
	Check    string `json:"check"`
	Severity string `json:"severity"`
	Entity   string `json:"entity"`
	Message  string `json:"message"`
}

// ValidateSummary holds aggregated counts for validate output.
type ValidateSummary struct {
	TotalIssues int            `json:"total_issues"`
	BySeverity  map[string]int `json:"by_severity"`
}
