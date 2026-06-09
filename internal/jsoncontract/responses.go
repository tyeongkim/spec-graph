package jsoncontract

import (
	"encoding/json"

	"github.com/tyeongkim/spec-graph/internal/model"
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
	Valid        bool                        `json:"valid"`
	Issues       []ValidateIssue             `json:"issues"`
	Summary      ValidateSummary             `json:"summary"`
	Satisfaction []ValidatePhaseSatisfaction `json:"satisfaction,omitempty"`
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

// ValidateSatisfactionItem is a per-entity satisfaction outcome.
//
// Status is one of "satisfied", "unsatisfied", or "advisory". Advisory items
// are reported but never block validation. EvidenceID names the source entity
// providing evidence (e.g. the phase that delivers a requirement, or the
// decision that answers a question). EvidenceRelation is the inbound relation
// type used as evidence: "delivers" for requirements, "answers" for questions,
// "mitigates" for risks. Both evidence fields are omitted when satisfaction
// was determined by the member's own status (assumption/decision rules) or
// when no evidence relation was found.
type ValidateSatisfactionItem struct {
	EntityID         string `json:"entity_id"`
	EntityType       string `json:"entity_type"`
	Status           string `json:"status"`
	Reason           string `json:"reason"`
	EvidenceID       string `json:"evidence_id,omitempty"`
	EvidenceRelation string `json:"evidence_relation,omitempty"`
}

// ValidatePhaseSatisfaction is the satisfaction report for a single phase.
//
// Satisfied/Total reflect the mandatory closure ratio (members reached via
// covers + 1-depth depends_on outbound + 1-depth implements inbound).
// AdvisoryCount counts members reached via opt-in references; advisory members
// never affect Satisfied or Total. Items lists every closure member with its
// per-entity outcome.
type ValidatePhaseSatisfaction struct {
	PhaseID       string                     `json:"phase_id"`
	Satisfied     int                        `json:"satisfied"`
	Total         int                        `json:"total"`
	AdvisoryCount int                        `json:"advisory_count"`
	Items         []ValidateSatisfactionItem `json:"items"`
}

// EntityUpdateGateResponse is returned when a status transition is blocked
// by validation gates. It wraps the blocking issues and warnings so consumers
// can parse them with the same schema as validate output.
type EntityUpdateGateResponse struct {
	Blocked    bool            `json:"blocked"`
	EntityID   string          `json:"entity_id"`
	EntityType string          `json:"entity_type"`
	FromStatus string          `json:"from_status"`
	ToStatus   string          `json:"to_status"`
	Issues     []ValidateIssue `json:"issues"`
	Warnings   []ValidateIssue `json:"warnings"`
	Summary    ValidateSummary `json:"summary"`
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
	Reason      string           `json:"reason,omitempty"`
	Actor       string           `json:"actor,omitempty"`
	Detail      string           `json:"detail,omitempty"`
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
	Layer      string  `json:"layer"`
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

type ExportJSONEntity struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Title    string                 `json:"title"`
	Status   string                 `json:"status"`
	Layer    string                 `json:"layer"`
	Metadata map[string]interface{} `json:"metadata"`
}

type ExportJSONRelation struct {
	FromID string  `json:"from_id"`
	ToID   string  `json:"to_id"`
	Type   string  `json:"type"`
	Layer  string  `json:"layer"`
	Weight float64 `json:"weight"`
}

type ExportJSONResult struct {
	Entities  []ExportJSONEntity   `json:"entities"`
	Relations []ExportJSONRelation `json:"relations"`
}

type PhaseNextResponse struct {
	Phase     PhaseNextDetail `json:"phase"`
	Scope     PhaseNextScope  `json:"scope"`
	Activated bool            `json:"activated"`
}

type PhaseNextDetail struct {
	ID                   string          `json:"id"`
	Title                string          `json:"title"`
	Status               string          `json:"status"`
	Goal                 string          `json:"goal"`
	Order                float64         `json:"order"`
	PredecessorsResolved bool            `json:"predecessors_resolved"`
	Metadata             json.RawMessage `json:"metadata"`
}

type PhaseNextScope struct {
	Total     int      `json:"total"`
	Delivered int      `json:"delivered"`
	Remaining []string `json:"remaining"`
}

type DoctorIssue struct {
	File    string `json:"file"`
	Message string `json:"message"`
}

type DoctorCheck struct {
	Name   string        `json:"name"`
	Status string        `json:"status"`
	Issues []DoctorIssue `json:"issues"`
}

type DoctorSummary struct {
	TotalChecks int `json:"total_checks"`
	Passed      int `json:"passed"`
	Failed      int `json:"failed"`
	TotalIssues int `json:"total_issues"`
}

type DoctorReport struct {
	Healthy bool          `json:"healthy"`
	Checks  []DoctorCheck `json:"checks"`
	Summary DoctorSummary `json:"summary"`
}

type MigrateResult struct {
	Migrated  bool   `json:"migrated,omitempty"`
	DryRun    bool   `json:"dry_run,omitempty"`
	Entities  int    `json:"entities"`
	Relations int    `json:"relations"`
	Backup    string `json:"backup,omitempty"`
	TOMLRoot  string `json:"toml_root,omitempty"`
}
