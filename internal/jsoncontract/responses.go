package jsoncontract

import (
	"encoding/json"

	"github.com/taeyeong/spec-graph/internal/model"
)

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

type ChangesetDetail struct {
	ID        string `json:"id"`
	Reason    string `json:"reason"`
	Actor     string `json:"actor,omitempty"`
	Source    string `json:"source,omitempty"`
	CreatedAt string `json:"created_at"`
}

type ChangesetResponse struct {
	Changeset       ChangesetDetail        `json:"changeset"`
	EntityEntries   []EntityHistoryEntry   `json:"entity_entries"`
	RelationEntries []RelationHistoryEntry `json:"relation_entries"`
}

type ChangesetListResponse struct {
	Changesets []ChangesetDetail `json:"changesets"`
	Count      int               `json:"count"`
}

type EntityHistoryEntry struct {
	ID          int              `json:"id"`
	ChangesetID string           `json:"changeset_id"`
	EntityID    string           `json:"entity_id"`
	Action      string           `json:"action"`
	Before      *json.RawMessage `json:"before"`
	After       *json.RawMessage `json:"after"`
	CreatedAt   string           `json:"created_at"`
}

type EntityHistoryResponse struct {
	EntityID string               `json:"entity_id"`
	Entries  []EntityHistoryEntry `json:"entries"`
	Count    int                  `json:"count"`
}

type RelationHistoryEntry struct {
	ID          int              `json:"id"`
	ChangesetID string           `json:"changeset_id"`
	RelationKey string           `json:"relation_key"`
	Action      string           `json:"action"`
	Before      *json.RawMessage `json:"before"`
	After       *json.RawMessage `json:"after"`
	CreatedAt   string           `json:"created_at"`
}

type RelationHistoryResponse struct {
	RelationKey string                 `json:"relation_key"`
	Entries     []RelationHistoryEntry `json:"entries"`
	Count       int                    `json:"count"`
}

type BootstrapEntityCandidate struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Title      string  `json:"title"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"`
}

type BootstrapRelationCandidate struct {
	From       string  `json:"from"`
	To         string  `json:"to"`
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"`
}

type BootstrapScanResponse struct {
	Entities  []BootstrapEntityCandidate   `json:"entities"`
	Relations []BootstrapRelationCandidate `json:"relations"`
}

type BootstrapSkippedItem struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type BootstrapErrorItem struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

type BootstrapImportResponse struct {
	Created []string               `json:"created"`
	Skipped []BootstrapSkippedItem `json:"skipped"`
	Errors  []BootstrapErrorItem   `json:"errors"`
}

// EntitySummary is a lightweight entity representation used in query responses.
type EntitySummary struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// RelationSummary describes a single relation edge in query responses.
type RelationSummary struct {
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
	Type   string `json:"type"`
}

// PathStep represents one node in a path traversal result.
type PathStep struct {
	EntityID   string `json:"entity_id"`
	EntityType string `json:"entity_type"`
	Relation   string `json:"relation"`
}

// QueryScopeSummary holds aggregated counts for query scope output.
type QueryScopeSummary struct {
	Total  int            `json:"total"`
	ByType map[string]int `json:"by_type"`
}

// QueryUnresolvedSummary holds aggregated counts for unresolved query output.
type QueryUnresolvedSummary struct {
	Total  int            `json:"total"`
	ByType map[string]int `json:"by_type"`
}

// QueryScopeResponse is the top-level JSON response for the query scope command.
type QueryScopeResponse struct {
	PhaseID   string            `json:"phase_id"`
	Entities  []EntitySummary   `json:"entities"`
	Relations []RelationSummary `json:"relations"`
	Summary   QueryScopeSummary `json:"summary"`
}

// QueryPathResponse is the top-level JSON response for the query path command.
type QueryPathResponse struct {
	From   string     `json:"from"`
	To     string     `json:"to"`
	Found  bool       `json:"found"`
	Path   []PathStep `json:"path"`
	Length int        `json:"length"`
}

// QueryUnresolvedResponse is the top-level JSON response for the query unresolved command.
type QueryUnresolvedResponse struct {
	Entities []EntitySummary        `json:"entities"`
	Summary  QueryUnresolvedSummary `json:"summary"`
}

type NeighborEntityResponse struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Depth  int    `json:"depth"`
}

type QueryNeighborsResponse struct {
	Center    string                   `json:"center"`
	Entities  []NeighborEntityResponse `json:"entities"`
	Relations []RelationSummary        `json:"relations"`
}

// QuerySQLResponse is the top-level JSON response for the query sql command.
type QuerySQLResponse struct {
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
	Count   int                      `json:"count"`
}

// ExportResponse is the top-level JSON response for the export command.
type ExportResponse struct {
	Format string `json:"format"`
	Output string `json:"output"`
}
