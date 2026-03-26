package graph

import (
	"container/heap"
	"fmt"

	"github.com/taeyeong/spec-graph/internal/model"
)

// pqItem is a single entry in the impact-analysis priority queue.
type pqItem struct {
	nodeID string
	score  float64 // max dimension score — used for priority ordering
	index  int     // managed by container/heap
}

// priorityQueue implements heap.Interface as a max-heap ordered by score.
type priorityQueue []*pqItem

func (pq priorityQueue) Len() int           { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool { return pq[i].score > pq[j].score } // max-heap
func (pq priorityQueue) Swap(i, j int)      { pq[i], pq[j] = pq[j], pq[i]; pq[i].index = i; pq[j].index = j }

func (pq *priorityQueue) Push(x any) {
	item := x.(*pqItem)
	item.index = len(*pq)
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[:n-1]
	return item
}

// visitedEntry tracks the best-known scores and path for a visited node.
type visitedEntry struct {
	scores        DimensionScores
	path          []string
	relationChain []model.RelationType
	depth         int
}

// maxDim returns the maximum of the three dimension scores.
func maxDim(s DimensionScores) float64 {
	m := s.Structural
	if s.Behavioral > m {
		m = s.Behavioral
	}
	if s.Planning > m {
		m = s.Planning
	}
	return m
}

// Impact performs priority-queue BFS impact analysis starting from the given
// source entity IDs. It returns all transitively affected entities with
// per-dimension scores, severity, and path information.
func Impact(sources []string, opts ImpactOptions, rf RelationFetcher, ef EntityFetcher) (*ImpactResult, error) {
	if len(sources) == 0 {
		return &ImpactResult{
			Sources:  sources,
			Affected: []AffectedEntity{},
			Summary:  ImpactSummary{Total: 0, ByType: map[model.EntityType]int{}, ByImpact: map[Severity]int{}},
		}, nil
	}

	var followSet map[model.RelationType]bool
	if opts.Follow != nil {
		followSet = make(map[model.RelationType]bool, len(opts.Follow))
		for _, rt := range opts.Follow {
			followSet[rt] = true
		}
	}

	sourceSet := make(map[string]bool, len(sources))
	for _, s := range sources {
		sourceSet[s] = true
	}

	visited := make(map[string]*visitedEntry)

	// Seed the priority queue with sources.
	pq := &priorityQueue{}
	heap.Init(pq)

	for _, src := range sources {
		scores := DimensionScores{Structural: 1.0, Behavioral: 1.0, Planning: 1.0}
		if opts.Dimension != nil {
			scores = singleDimension(*opts.Dimension, 1.0)
		}
		visited[src] = &visitedEntry{
			scores:        scores,
			path:          []string{src},
			relationChain: nil,
			depth:         0,
		}
		heap.Push(pq, &pqItem{nodeID: src, score: maxDim(scores)})
	}

	// BFS with priority queue.
	for pq.Len() > 0 {
		item := heap.Pop(pq).(*pqItem)
		current := item.nodeID
		entry := visited[current]

		// Stale entry check: if the popped score is worse than what we have, skip.
		if item.score < maxDim(entry.scores) {
			continue
		}

		rels, err := rf.GetByEntity(current)
		if err != nil {
			return nil, fmt.Errorf("fetching relations for %s: %w", current, err)
		}

		for _, rel := range rels {
			if followSet != nil && !followSet[rel.Type] {
				continue
			}

			if opts.Layer != nil && model.LayerForRelationType(rel.Type) != *opts.Layer {
				continue
			}

			rule, ok := PropagationTable[rel.Type]
			if !ok {
				continue
			}

			neighbor, reverse := resolveNeighbor(current, rel, rule.Direction)
			if neighbor == "" || sourceSet[neighbor] {
				continue
			}

			newScores := computeScores(entry.scores, rule.Scores, rel.Weight, reverse, opts.Dimension)

			best := maxDim(newScores)
			if best <= 0 {
				continue
			}

			existing, seen := visited[neighbor]
			if seen && maxDim(existing.scores) >= best {
				continue
			}

			// Build new path and relation chain.
			newPath := make([]string, len(entry.path)+1)
			copy(newPath, entry.path)
			newPath[len(entry.path)] = neighbor

			newChain := make([]model.RelationType, len(entry.relationChain)+1)
			copy(newChain, entry.relationChain)
			newChain[len(entry.relationChain)] = rel.Type

			visited[neighbor] = &visitedEntry{
				scores:        newScores,
				path:          newPath,
				relationChain: newChain,
				depth:         entry.depth + 1,
			}

			heap.Push(pq, &pqItem{nodeID: neighbor, score: best})
		}
	}

	// Build result from visited entries (excluding sources).
	affected := make([]AffectedEntity, 0, len(visited))
	for id, entry := range visited {
		if sourceSet[id] {
			continue
		}

		overall := OverallSeverity(entry.scores)

		// Apply min-severity filter.
		if opts.MinSeverity != nil && !meetsMinSeverity(overall, *opts.MinSeverity) {
			continue
		}

		ent, err := ef.Get(id)
		if err != nil {
			return nil, fmt.Errorf("fetching entity %s: %w", id, err)
		}

		reason := ""
		if len(entry.relationChain) > 0 {
			lastRel := entry.relationChain[len(entry.relationChain)-1]
			if r, ok := ReasonTemplates[lastRel]; ok {
				reason = r
			}
		}

		affected = append(affected, AffectedEntity{
			ID:            id,
			Type:          ent.Type,
			Depth:         entry.depth,
			Path:          entry.path,
			RelationChain: entry.relationChain,
			Impact:        entry.scores,
			Overall:       overall,
			Reason:        reason,
		})
	}

	// Build summary.
	summary := ImpactSummary{
		Total:    len(affected),
		ByType:   make(map[model.EntityType]int),
		ByImpact: make(map[Severity]int),
	}
	for _, a := range affected {
		summary.ByType[a.Type]++
		summary.ByImpact[a.Overall]++
	}

	return &ImpactResult{
		Sources:  sources,
		Affected: affected,
		Summary:  summary,
	}, nil
}

// resolveNeighbor determines the neighbor node and whether propagation is
// in the reverse direction, based on the relation direction rule.
// Returns ("", false) if propagation is not allowed in this direction.
func resolveNeighbor(current string, rel model.Relation, dir PropagationDirection) (neighbor string, reverse bool) {
	switch dir {
	case Forward:
		if current == rel.FromID {
			return rel.ToID, false
		}
		return "", false

	case Bidirectional:
		if current == rel.FromID {
			return rel.ToID, false
		}
		return rel.FromID, false

	case ForwardReverseWeak:
		if current == rel.FromID {
			return rel.ToID, false
		}
		if current == rel.ToID {
			return rel.FromID, true
		}
		return "", false
	}
	return "", false
}

// computeScores calculates new dimension scores for a neighbor node.
func computeScores(parent, prop DimensionScores, relWeight float64, reverse bool, dimension *string) DimensionScores {
	factor := 1.0
	if reverse {
		factor = ReverseWeakFactor
	}

	if dimension != nil {
		return singleDimensionCompute(*dimension, parent, prop, relWeight, factor)
	}

	return DimensionScores{
		Structural: parent.Structural * prop.Structural * relWeight * factor,
		Behavioral: parent.Behavioral * prop.Behavioral * relWeight * factor,
		Planning:   parent.Planning * prop.Planning * relWeight * factor,
	}
}

// singleDimension returns DimensionScores with only the named dimension set.
func singleDimension(dim string, value float64) DimensionScores {
	var s DimensionScores
	switch dim {
	case "structural":
		s.Structural = value
	case "behavioral":
		s.Behavioral = value
	case "planning":
		s.Planning = value
	}
	return s
}

// singleDimensionCompute computes scores for a single dimension only.
func singleDimensionCompute(dim string, parent, prop DimensionScores, relWeight, factor float64) DimensionScores {
	var s DimensionScores
	switch dim {
	case "structural":
		s.Structural = parent.Structural * prop.Structural * relWeight * factor
	case "behavioral":
		s.Behavioral = parent.Behavioral * prop.Behavioral * relWeight * factor
	case "planning":
		s.Planning = parent.Planning * prop.Planning * relWeight * factor
	}
	return s
}

// meetsMinSeverity returns true if actual severity meets or exceeds the minimum.
func meetsMinSeverity(actual, min Severity) bool {
	return severityRank(actual) >= severityRank(min)
}

func severityRank(s Severity) int {
	switch s {
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	default:
		return 0
	}
}
