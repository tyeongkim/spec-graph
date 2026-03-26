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
	// Layer restricts traversal to entities in this layer. nil = all layers.
	Layer *model.Layer
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

// QueryScopeOptions controls which phase to scope the query to.
type QueryScopeOptions struct {
	// PhaseID is the ID of the phase entity to scope by.
	PhaseID string
	// Layer restricts scoped entities to this layer. nil = all layers.
	Layer *model.Layer
}

// QueryScopeResult holds entities and relations belonging to a phase scope.
type QueryScopeResult struct {
	PhaseID   string           `json:"phase_id"`
	Entities  []model.Entity   `json:"entities"`
	Relations []model.Relation `json:"relations"`
}

// QueryPathOptions specifies the source and destination for a path query.
type QueryPathOptions struct {
	// FromID is the starting entity ID.
	FromID string
	// ToID is the target entity ID.
	ToID string
	// Layer restricts path traversal to entities in this layer. nil = all layers.
	Layer *model.Layer
}

// PathNode represents a single step in a traversal path.
type PathNode struct {
	EntityID   string           `json:"entity_id"`
	EntityType model.EntityType `json:"entity_type"`
	// Relation is the relation type used to reach this node (empty for the first node).
	Relation model.RelationType `json:"relation"`
}

// QueryPathResult holds the result of a path query between two entities.
type QueryPathResult struct {
	FromID string     `json:"from_id"`
	ToID   string     `json:"to_id"`
	Path   []PathNode `json:"path"`
	Found  bool       `json:"found"`
}

// NeighborEntity pairs an entity with its BFS depth from the center.
type NeighborEntity struct {
	Entity model.Entity
	Depth  int
}

// NeighborResult holds the result of a neighbor traversal from a center entity.
type NeighborResult struct {
	Center    string
	Entities  []NeighborEntity
	Relations []model.Relation
}

// QueryUnresolvedOptions controls optional filtering for unresolved entity queries.
type QueryUnresolvedOptions struct {
	// Type filters results to a specific entity type. nil = all types.
	Type *model.EntityType
}

// QueryUnresolvedResult holds entities that are in an unresolved (draft) state.
type QueryUnresolvedResult struct {
	Entities []model.Entity `json:"entities"`
	Count    int            `json:"count"`
}
