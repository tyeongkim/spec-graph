package graph

import (
	"container/heap"
	"fmt"
	"slices"

	"github.com/tyeongkim/spec-graph/internal/model"
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

// Push accepts any because heap.Interface requires it. Only popItem and this
// package construct queue entries, and both use *pqItem, so the assertion holds.
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

// popItem removes the highest-scoring entry. It concentrates the assertion that
// heap.Pop's any is a *pqItem into one place; Push only ever accepts *pqItem.
func popItem(pq *priorityQueue) *pqItem {
	return heap.Pop(pq).(*pqItem)
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

	sourceSet := make(map[string]bool, len(sources))
	for _, s := range sources {
		sourceSet[s] = true
	}

	visited, err := propagate(sources, sourceSet, opts, rf)
	if err != nil {
		return nil, err
	}

	affected, err := collectAffected(visited, sourceSet, opts, ef)
	if err != nil {
		return nil, err
	}

	return &ImpactResult{
		Sources:  sources,
		Affected: affected,
		Summary:  summarize(affected),
	}, nil
}

// propagate walks the graph outward from the sources, keeping the highest score
// found for each node. Nodes are expanded highest-score-first so that a node is
// usually settled before it is expanded, and re-queued entries with a stale
// score are discarded on pop.
func propagate(sources []string, sourceSet map[string]bool, opts ImpactOptions, rf RelationFetcher) (map[string]*visitedEntry, error) {
	followSet := followFilter(opts.Follow)
	visited := make(map[string]*visitedEntry)

	pq := &priorityQueue{}
	heap.Init(pq)
	for _, src := range sources {
		scores := DimensionScores{Structural: 1.0, Behavioral: 1.0, Planning: 1.0}
		if opts.Dimension != nil {
			scores = singleDimension(*opts.Dimension, 1.0)
		}
		visited[src] = &visitedEntry{
			scores: scores,
			path:   []string{src},
			depth:  0,
		}
		heap.Push(pq, &pqItem{nodeID: src, score: maxDim(scores)})
	}

	for pq.Len() > 0 {
		item := popItem(pq)
		current := item.nodeID
		entry := visited[current]

		if item.score < maxDim(entry.scores) {
			continue
		}

		rels, err := rf.GetByEntity(current)
		if err != nil {
			return nil, fmt.Errorf("fetching relations for %s: %w", current, err)
		}

		for _, rel := range rels {
			neighbor, scores, ok := propagateAlong(current, rel, entry, followSet, sourceSet, opts)
			if !ok {
				continue
			}

			existing, seen := visited[neighbor]
			best := maxDim(scores)
			if seen && maxDim(existing.scores) >= best {
				continue
			}

			visited[neighbor] = &visitedEntry{
				scores:        scores,
				path:          append(slices.Clone(entry.path), neighbor),
				relationChain: append(slices.Clone(entry.relationChain), rel.Type),
				depth:         entry.depth + 1,
			}
			heap.Push(pq, &pqItem{nodeID: neighbor, score: best})
		}
	}

	return visited, nil
}

// propagateAlong reports the neighbour reached by following rel out of current,
// with the scores it inherits. ok is false when the relation is filtered out,
// carries no propagation rule, or leads nowhere new.
func propagateAlong(
	current string,
	rel model.Relation,
	entry *visitedEntry,
	followSet map[model.RelationType]bool,
	sourceSet map[string]bool,
	opts ImpactOptions,
) (neighbor string, scores DimensionScores, ok bool) {
	if followSet != nil && !followSet[rel.Type] {
		return "", DimensionScores{}, false
	}
	if opts.Layer != nil && model.LayerForRelationType(rel.Type) != *opts.Layer {
		return "", DimensionScores{}, false
	}

	rule, known := PropagationTable[rel.Type]
	if !known {
		return "", DimensionScores{}, false
	}

	neighbor, reverse := resolveNeighbor(current, rel, rule.Direction)
	if neighbor == "" || sourceSet[neighbor] {
		return "", DimensionScores{}, false
	}

	scores = computeScores(entry.scores, rule.Scores, rel.Weight, reverse, opts.Dimension)
	if maxDim(scores) <= 0 {
		return "", DimensionScores{}, false
	}

	return neighbor, scores, true
}

// followFilter builds the set of relation types to traverse. A nil result means
// every relation type is followed.
func followFilter(follow []model.RelationType) map[model.RelationType]bool {
	if follow == nil {
		return nil
	}
	set := make(map[model.RelationType]bool, len(follow))
	for _, rt := range follow {
		set[rt] = true
	}
	return set
}

// collectAffected turns the traversal result into the reported entities,
// dropping the sources themselves and anything below the severity floor.
func collectAffected(visited map[string]*visitedEntry, sourceSet map[string]bool, opts ImpactOptions, ef EntityFetcher) ([]AffectedEntity, error) {
	affected := make([]AffectedEntity, 0, len(visited))

	for id, entry := range visited {
		if sourceSet[id] {
			continue
		}

		overall := OverallSeverity(entry.scores)
		if opts.MinSeverity != nil && !meetsMinSeverity(overall, *opts.MinSeverity) {
			continue
		}

		ent, err := ef.Get(id)
		if err != nil {
			return nil, fmt.Errorf("fetching entity %s: %w", id, err)
		}

		affected = append(affected, AffectedEntity{
			ID:            id,
			Type:          ent.Type,
			Depth:         entry.depth,
			Path:          entry.path,
			RelationChain: entry.relationChain,
			Impact:        entry.scores,
			Overall:       overall,
			Reason:        reasonFor(entry.relationChain),
		})
	}

	return affected, nil
}

// reasonFor explains an entity's inclusion using the last relation traversed to
// reach it.
func reasonFor(chain []model.RelationType) string {
	if len(chain) == 0 {
		return ""
	}
	return ReasonTemplates[chain[len(chain)-1]]
}

func summarize(affected []AffectedEntity) ImpactSummary {
	summary := ImpactSummary{
		Total:    len(affected),
		ByType:   make(map[model.EntityType]int),
		ByImpact: make(map[Severity]int),
	}
	for _, a := range affected {
		summary.ByType[a.Type]++
		summary.ByImpact[a.Overall]++
	}
	return summary
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
func computeScores(parent, prop DimensionScores, relWeight float64, reverse bool, dimension *Dimension) DimensionScores {
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
func singleDimension(dim Dimension, value float64) DimensionScores {
	var s DimensionScores
	switch dim {
	case DimensionStructural:
		s.Structural = value
	case DimensionBehavioral:
		s.Behavioral = value
	case DimensionPlanning:
		s.Planning = value
	}
	return s
}

// singleDimensionCompute computes scores for a single dimension only.
func singleDimensionCompute(dim Dimension, parent, prop DimensionScores, relWeight, factor float64) DimensionScores {
	var s DimensionScores
	switch dim {
	case DimensionStructural:
		s.Structural = parent.Structural * prop.Structural * relWeight * factor
	case DimensionBehavioral:
		s.Behavioral = parent.Behavioral * prop.Behavioral * relWeight * factor
	case DimensionPlanning:
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
