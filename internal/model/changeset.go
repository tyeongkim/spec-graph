package model

type HistoryAction string

const (
	ActionCreate    HistoryAction = "create"
	ActionUpdate    HistoryAction = "update"
	ActionDeprecate HistoryAction = "deprecate"
	ActionDelete    HistoryAction = "delete"
)

type Changeset struct {
	ID        string `json:"id"`
	Reason    string `json:"reason"`
	Actor     string `json:"actor,omitempty"`
	Source    string `json:"source,omitempty"`
	CreatedAt string `json:"created_at"`
}

type EntityHistoryEntry struct {
	ID          int           `json:"id"`
	ChangesetID string        `json:"changeset_id"`
	EntityID    string        `json:"entity_id"`
	Action      HistoryAction `json:"action"`
	BeforeJSON  *string       `json:"before_json,omitempty"`
	AfterJSON   *string       `json:"after_json,omitempty"`
	CreatedAt   string        `json:"created_at"`
}

type RelationHistoryEntry struct {
	ID          int           `json:"id"`
	ChangesetID string        `json:"changeset_id"`
	RelationKey string        `json:"relation_key"`
	Action      HistoryAction `json:"action"`
	BeforeJSON  *string       `json:"before_json,omitempty"`
	AfterJSON   *string       `json:"after_json,omitempty"`
	CreatedAt   string        `json:"created_at"`
}
