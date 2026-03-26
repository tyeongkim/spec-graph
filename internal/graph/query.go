package graph

import (
	"fmt"
	"sort"

	"github.com/taeyeong/spec-graph/internal/model"
)

// QueryScope returns all entities and relations belonging to the given phase.
// It finds covers/delivers relations from the phase to arch entities.
func QueryScope(opts QueryScopeOptions, rf RelationFetcher, ef EntityFetcher) (*QueryScopeResult, error) {
	// Verify phase entity exists and is of type "phase".
	phase, err := ef.Get(opts.PhaseID)
	if err != nil {
		return nil, err
	}
	if phase.Type != model.EntityTypePhase {
		return nil, &model.ErrInvalidInput{
			Message: fmt.Sprintf("entity %q is type %q, not phase", opts.PhaseID, phase.Type),
		}
	}

	// Get all relations where the phase is involved.
	rels, err := rf.GetByEntity(opts.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("fetching relations for phase %s: %w", opts.PhaseID, err)
	}

	// Filter to covers / delivers (phase→arch).
	var matchedRels []model.Relation
	entityIDs := make(map[string]bool)

	for _, rel := range rels {
		if rel.FromID == opts.PhaseID &&
			(rel.Type == model.RelationCovers || rel.Type == model.RelationDelivers) {
			matchedRels = append(matchedRels, rel)
			entityIDs[rel.ToID] = true
		}
	}

	entities := make([]model.Entity, 0, len(entityIDs))
	for id := range entityIDs {
		ent, err := ef.Get(id)
		if err != nil {
			return nil, fmt.Errorf("fetching entity %s: %w", id, err)
		}
		if opts.Layer != nil && model.LayerForEntityType(ent.Type) != *opts.Layer {
			continue
		}
		entities = append(entities, ent)
	}

	if matchedRels == nil {
		matchedRels = []model.Relation{}
	}

	return &QueryScopeResult{
		PhaseID:   opts.PhaseID,
		Entities:  entities,
		Relations: matchedRels,
	}, nil
}

// unresolvedTypes lists the entity types considered "unresolved".
var unresolvedTypes = []model.EntityType{
	model.EntityTypeQuestion,
	model.EntityTypeAssumption,
	model.EntityTypeRisk,
}

// QueryUnresolved returns entities of type question, assumption, or risk that
// have status draft or active. If opts.Type is set, only that type is returned.
// Results are sorted by type then by ID.
func QueryUnresolved(opts QueryUnresolvedOptions, ef EntityFetcher) (*QueryUnresolvedResult, error) {
	types := unresolvedTypes
	if opts.Type != nil {
		types = []model.EntityType{*opts.Type}
	}

	var all []model.Entity
	for _, t := range types {
		entities, err := ef.List(EntityListFilters{Type: &t})
		if err != nil {
			return nil, err
		}
		for _, e := range entities {
			if e.Status == model.EntityStatusDraft || e.Status == model.EntityStatusActive {
				all = append(all, e)
			}
		}
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].Type != all[j].Type {
			return all[i].Type < all[j].Type
		}
		return all[i].ID < all[j].ID
	})

	if all == nil {
		all = []model.Entity{}
	}

	return &QueryUnresolvedResult{
		Entities: all,
		Count:    len(all),
	}, nil
}

const maxPathDepth = 50

type bfsNode struct {
	entityID   string
	parentID   string
	relation   model.RelationType
	entityType model.EntityType
}

// QueryPath performs a BFS from opts.FromID to opts.ToID across all relation
// types (bidirectional traversal). It returns the shortest path if one exists.
func QueryPath(opts QueryPathOptions, rf RelationFetcher, ef EntityFetcher) (*QueryPathResult, error) {
	fromEntity, err := ef.Get(opts.FromID)
	if err != nil {
		return nil, fmt.Errorf("source entity: %w", err)
	}
	toEntity, err := ef.Get(opts.ToID)
	if err != nil {
		return nil, fmt.Errorf("target entity: %w", err)
	}

	if opts.FromID == opts.ToID {
		return &QueryPathResult{
			FromID: opts.FromID,
			ToID:   opts.ToID,
			Found:  true,
			Path: []PathNode{
				{EntityID: opts.FromID, EntityType: fromEntity.Type},
			},
		}, nil
	}

	visited := map[string]*bfsNode{
		opts.FromID: {
			entityID:   opts.FromID,
			parentID:   "",
			entityType: fromEntity.Type,
		},
	}
	queue := []string{opts.FromID}
	_ = toEntity

	for depth := 0; depth < maxPathDepth && len(queue) > 0; depth++ {
		nextQueue := make([]string, 0)

		for _, current := range queue {
			rels, relErr := rf.GetByEntity(current)
			if relErr != nil {
				return nil, fmt.Errorf("fetching relations for %s: %w", current, relErr)
			}

			for _, rel := range rels {
				if opts.Layer != nil && model.LayerForRelationType(rel.Type) != *opts.Layer {
					continue
				}

				var neighbor string
				if rel.FromID == current {
					neighbor = rel.ToID
				} else {
					neighbor = rel.FromID
				}

				if _, seen := visited[neighbor]; seen {
					continue
				}

				ent, entErr := ef.Get(neighbor)
				if entErr != nil {
					continue
				}

				visited[neighbor] = &bfsNode{
					entityID:   neighbor,
					parentID:   current,
					relation:   rel.Type,
					entityType: ent.Type,
				}

				if neighbor == opts.ToID {
					return &QueryPathResult{
						FromID: opts.FromID,
						ToID:   opts.ToID,
						Found:  true,
						Path:   reconstructPath(visited, opts.FromID, opts.ToID),
					}, nil
				}

				nextQueue = append(nextQueue, neighbor)
			}
		}

		queue = nextQueue
	}

	return &QueryPathResult{
		FromID: opts.FromID,
		ToID:   opts.ToID,
		Found:  false,
		Path:   []PathNode{},
	}, nil
}

// Neighbors performs a BFS from entityID up to the given depth, traversing
// relations in both directions. Depth 0 returns only the center entity.
func Neighbors(entityID string, depth int, rf RelationFetcher, ef EntityFetcher) (*NeighborResult, error) {
	center, err := ef.Get(entityID)
	if err != nil {
		return nil, fmt.Errorf("center entity: %w", err)
	}

	visited := map[string]int{entityID: 0}
	entities := []NeighborEntity{{Entity: center, Depth: 0}}
	var relations []model.Relation
	relSeen := make(map[string]bool)

	queue := []string{entityID}

	for d := 0; d < depth && len(queue) > 0; d++ {
		var nextQueue []string

		for _, current := range queue {
			rels, relErr := rf.GetByEntity(current)
			if relErr != nil {
				return nil, fmt.Errorf("fetching relations for %s: %w", current, relErr)
			}

			for _, rel := range rels {
				relKey := rel.FromID + "|" + string(rel.Type) + "|" + rel.ToID
				if !relSeen[relKey] {
					relSeen[relKey] = true
					relations = append(relations, rel)
				}

				var neighbor string
				if rel.FromID == current {
					neighbor = rel.ToID
				} else {
					neighbor = rel.FromID
				}

				if _, seen := visited[neighbor]; seen {
					continue
				}

				ent, entErr := ef.Get(neighbor)
				if entErr != nil {
					continue
				}

				visited[neighbor] = d + 1
				entities = append(entities, NeighborEntity{Entity: ent, Depth: d + 1})
				nextQueue = append(nextQueue, neighbor)
			}
		}

		queue = nextQueue
	}

	// Collect relations between already-visited nodes that we haven't seen yet
	// (edges within the last depth layer).
	for _, current := range queue {
		rels, relErr := rf.GetByEntity(current)
		if relErr != nil {
			continue
		}
		for _, rel := range rels {
			_, fromIn := visited[rel.FromID]
			_, toIn := visited[rel.ToID]
			if fromIn && toIn {
				relKey := rel.FromID + "|" + string(rel.Type) + "|" + rel.ToID
				if !relSeen[relKey] {
					relSeen[relKey] = true
					relations = append(relations, rel)
				}
			}
		}
	}

	if relations == nil {
		relations = []model.Relation{}
	}

	return &NeighborResult{
		Center:    entityID,
		Entities:  entities,
		Relations: relations,
	}, nil
}

// reconstructPath walks parent pointers from toID back to fromID and reverses.
func reconstructPath(visited map[string]*bfsNode, fromID, toID string) []PathNode {
	var path []PathNode
	for current := toID; current != ""; {
		node := visited[current]
		path = append(path, PathNode{
			EntityID:   node.entityID,
			EntityType: node.entityType,
			Relation:   node.relation,
		})
		if current == fromID {
			break
		}
		current = node.parentID
	}

	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	if len(path) > 0 {
		path[0].Relation = ""
	}

	return path
}
