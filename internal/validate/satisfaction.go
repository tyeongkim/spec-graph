package validate

import (
	"fmt"
	"slices"
	"sort"

	"github.com/tyeongkim/spec-graph/internal/model"
)

var satisfactionEvidenceRelation = map[model.EntityType]model.RelationType{
	model.EntityTypeRequirement: model.RelationDelivers,
	model.EntityTypeQuestion:    model.RelationAnswers,
	model.EntityTypeRisk:        model.RelationMitigates,
}

var satisfactionStatusOnly = map[model.EntityType][]model.EntityStatus{
	model.EntityTypeAssumption: {model.EntityStatus("verified")},
	model.EntityTypeDecision:   {model.EntityStatusActive, model.EntityStatusResolved},
}

var satisfactionTargetStatusAllowlist = map[model.EntityType][]model.EntityStatus{
	model.EntityTypeDecision:    {model.EntityStatusActive, model.EntityStatusResolved},
	model.EntityTypeInterface:   {model.EntityStatusActive, model.EntityStatusResolved},
	model.EntityTypeTest:        {model.EntityStatus("verified"), model.EntityStatus("passed")},
	model.EntityTypeRequirement: {model.EntityStatusResolved, model.EntityStatus("verified")},
	model.EntityTypeRisk:        {model.EntityStatus("mitigated"), model.EntityStatusResolved},
	model.EntityTypePhase:       {model.EntityStatusActive, model.EntityStatus("completed"), model.EntityStatusResolved},
}

var fallbackTargetStatusAllowlist = []model.EntityStatus{
	model.EntityStatusActive,
	model.EntityStatusResolved,
}

type closureClassification int

const (
	closureMandatory closureClassification = iota
	closureAdvisory
)

type closureMember struct {
	entityID string
	class    closureClassification
	origin   model.RelationType
	from     string
}

// SatisfactionItemStatus represents the per-entity satisfaction outcome.
type SatisfactionItemStatus string

const (
	SatisfactionSatisfied   SatisfactionItemStatus = "satisfied"
	SatisfactionUnsatisfied SatisfactionItemStatus = "unsatisfied"
	SatisfactionAdvisory    SatisfactionItemStatus = "advisory"
)

// SatisfactionItem is one entry in the satisfaction report.
type SatisfactionItem struct {
	EntityID         string                 `json:"entity_id"`
	EntityType       model.EntityType       `json:"entity_type"`
	Status           SatisfactionItemStatus `json:"status"`
	Reason           string                 `json:"reason"`
	EvidenceID       string                 `json:"evidence_id,omitempty"`
	EvidenceRelation model.RelationType     `json:"evidence_relation,omitempty"`
}

// PhaseSatisfaction is the satisfaction report for a single phase.
type PhaseSatisfaction struct {
	PhaseID       string             `json:"phase_id"`
	Satisfied     int                `json:"satisfied"`
	Total         int                `json:"total"`
	AdvisoryCount int                `json:"advisory_count"`
	Items         []SatisfactionItem `json:"items"`
}

// computePhaseClosure builds the closure of arch entities reachable from a phase.
//
// Mandatory members:
//   - directly covered entities (phase --covers--> X)
//   - 1-depth depends_on outbound neighbors of covered entities (covered --depends_on--> Y)
//   - 1-depth implements inbound neighbors of covered entities (Z --implements--> covered)
//
// Advisory members (only when includeReferences is true):
//   - 1-depth references outbound neighbors (covered --references--> R)
//   - excluded if already mandatory (mandatory wins invariant)
func computePhaseClosure(phaseID string, includeReferences bool, rf RelationFetcher) ([]closureMember, error) {
	phaseRels, err := rf.GetByEntity(phaseID)
	if err != nil {
		return nil, fmt.Errorf("fetch phase relations: %w", err)
	}

	directlyCovered := make([]string, 0)
	covered := make(map[string]bool)
	for _, r := range phaseRels {
		if r.Type == model.RelationCovers && r.FromID == phaseID && !covered[r.ToID] {
			directlyCovered = append(directlyCovered, r.ToID)
			covered[r.ToID] = true
		}
	}

	mandatory := make(map[string]closureMember)
	for _, id := range directlyCovered {
		mandatory[id] = closureMember{
			entityID: id,
			class:    closureMandatory,
			origin:   model.RelationCovers,
			from:     phaseID,
		}
	}

	type pendingRef struct {
		entityID string
		origin   model.RelationType
		from     string
	}
	var refCandidates []pendingRef

	for _, id := range directlyCovered {
		rels, err := rf.GetByEntity(id)
		if err != nil {
			return nil, fmt.Errorf("fetch relations for %q: %w", id, err)
		}
		for _, r := range rels {
			switch r.Type {
			case model.RelationDependsOn:
				if r.FromID != id {
					continue
				}
				if _, exists := mandatory[r.ToID]; !exists {
					mandatory[r.ToID] = closureMember{
						entityID: r.ToID,
						class:    closureMandatory,
						origin:   r.Type,
						from:     id,
					}
				}
			case model.RelationImplements:
				if r.ToID != id {
					continue
				}
				if _, exists := mandatory[r.FromID]; !exists {
					mandatory[r.FromID] = closureMember{
						entityID: r.FromID,
						class:    closureMandatory,
						origin:   r.Type,
						from:     id,
					}
				}
			case model.RelationReferences:
				if !includeReferences {
					continue
				}
				if r.FromID != id {
					continue
				}
				refCandidates = append(refCandidates, pendingRef{
					entityID: r.ToID,
					origin:   r.Type,
					from:     id,
				})
			}
		}
	}

	advisory := make(map[string]closureMember)
	for _, c := range refCandidates {
		if _, isMandatory := mandatory[c.entityID]; isMandatory {
			continue
		}
		if _, exists := advisory[c.entityID]; exists {
			continue
		}
		advisory[c.entityID] = closureMember{
			entityID: c.entityID,
			class:    closureAdvisory,
			origin:   c.origin,
			from:     c.from,
		}
	}

	out := make([]closureMember, 0, len(mandatory)+len(advisory))
	for _, m := range mandatory {
		out = append(out, m)
	}
	for _, m := range advisory {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].class != out[j].class {
			return out[i].class == closureMandatory
		}
		return out[i].entityID < out[j].entityID
	})

	return out, nil
}

func statusInAllowlist(entityType model.EntityType, status model.EntityStatus) bool {
	allowed, _ := allowlistFor(entityType)
	return slices.Contains(allowed, status)
}

func allowlistFor(entityType model.EntityType) ([]model.EntityStatus, bool) {
	allowed, ok := satisfactionTargetStatusAllowlist[entityType]
	if !ok {
		return fallbackTargetStatusAllowlist, false
	}
	return allowed, true
}

// evaluateMember resolves a closure member to a satisfaction outcome.
//
// phaseID identifies the phase being validated; it is used to enforce that
// `delivers` evidence comes from THIS phase, not another phase that happens
// to also deliver the same entity. This is required for phase-exit gate
// semantics: PHS-1 covers REQ-1 is not satisfied by PHS-2 delivers REQ-1.
//
// Order of resolution:
//  1. Advisory class always returns advisory, no fetch required.
//  2. Mandatory members are fetched and judged:
//     a. Layer 2 (status-only rule, e.g. assumption/decision)
//     b. Layer 1 (existence of inbound evidence relation, scoped to this
//     phase when the relation is `delivers`) +
//     Layer 3 (any matching evidence source has status in per-type allowlist)
//     c. Fallback: type-specific status allowlist if no rule applies
func evaluateMember(member closureMember, phaseID string, ef EntityFetcher, rf RelationFetcher) SatisfactionItem {
	if member.class == closureAdvisory {
		entity, _ := ef.Get(member.entityID)
		return SatisfactionItem{
			EntityID:   member.entityID,
			EntityType: entity.Type,
			Status:     SatisfactionAdvisory,
			Reason: fmt.Sprintf("advisory inclusion via %s from %s",
				member.origin, member.from),
		}
	}

	entity, err := ef.Get(member.entityID)
	if err != nil {
		return SatisfactionItem{
			EntityID: member.entityID,
			Status:   SatisfactionUnsatisfied,
			Reason:   fmt.Sprintf("entity %q not found: %v", member.entityID, err),
		}
	}

	if allowed, ok := satisfactionStatusOnly[entity.Type]; ok {
		if slices.Contains(allowed, entity.Status) {
			return SatisfactionItem{
				EntityID:   entity.ID,
				EntityType: entity.Type,
				Status:     SatisfactionSatisfied,
				Reason:     fmt.Sprintf("status %q in allowlist", entity.Status),
			}
		}
		return SatisfactionItem{
			EntityID:   entity.ID,
			EntityType: entity.Type,
			Status:     SatisfactionUnsatisfied,
			Reason:     fmt.Sprintf("status %q not in allowlist %v", entity.Status, allowed),
		}
	}

	evidenceRel, hasEvidence := satisfactionEvidenceRelation[entity.Type]
	if !hasEvidence {
		allowed, _ := allowlistFor(entity.Type)
		if slices.Contains(allowed, entity.Status) {
			return SatisfactionItem{
				EntityID:   entity.ID,
				EntityType: entity.Type,
				Status:     SatisfactionSatisfied,
				Reason:     fmt.Sprintf("status %q in allowlist", entity.Status),
			}
		}
		return SatisfactionItem{
			EntityID:   entity.ID,
			EntityType: entity.Type,
			Status:     SatisfactionUnsatisfied,
			Reason:     fmt.Sprintf("status %q not in allowlist %v", entity.Status, allowed),
		}
	}

	rels, err := rf.GetByEntity(entity.ID)
	if err != nil {
		return SatisfactionItem{
			EntityID:         entity.ID,
			EntityType:       entity.Type,
			Status:           SatisfactionUnsatisfied,
			EvidenceRelation: evidenceRel,
			Reason:           fmt.Sprintf("fetch relations: %v", err),
		}
	}

	scopedToPhase := evidenceRel == model.RelationDelivers

	var sourceIDs []string
	var crossPhaseDelivers []string
	for _, r := range rels {
		if r.Type != evidenceRel || r.ToID != entity.ID {
			continue
		}
		if scopedToPhase && r.FromID != phaseID {
			crossPhaseDelivers = append(crossPhaseDelivers, r.FromID)
			continue
		}
		sourceIDs = append(sourceIDs, r.FromID)
	}

	if len(sourceIDs) == 0 {
		reason := fmt.Sprintf("no inbound %q relation", evidenceRel)
		if scopedToPhase && len(crossPhaseDelivers) > 0 {
			reason = fmt.Sprintf("no inbound %q relation from phase %s (found from %v)",
				evidenceRel, phaseID, crossPhaseDelivers)
		}
		return SatisfactionItem{
			EntityID:         entity.ID,
			EntityType:       entity.Type,
			Status:           SatisfactionUnsatisfied,
			EvidenceRelation: evidenceRel,
			Reason:           reason,
		}
	}

	var lastFailure SatisfactionItem
	lastFailure.EntityID = entity.ID
	lastFailure.EntityType = entity.Type
	lastFailure.Status = SatisfactionUnsatisfied
	lastFailure.EvidenceRelation = evidenceRel

	for _, sourceID := range sourceIDs {
		source, err := ef.Get(sourceID)
		if err != nil {
			lastFailure.EvidenceID = sourceID
			lastFailure.Reason = fmt.Sprintf("evidence source %q not found: %v", sourceID, err)
			continue
		}

		allowed, _ := allowlistFor(source.Type)
		if slices.Contains(allowed, source.Status) {
			return SatisfactionItem{
				EntityID:         entity.ID,
				EntityType:       entity.Type,
				Status:           SatisfactionSatisfied,
				EvidenceID:       source.ID,
				EvidenceRelation: evidenceRel,
				Reason: fmt.Sprintf("evidence %s (%q, status %q)",
					source.ID, evidenceRel, source.Status),
			}
		}

		lastFailure.EvidenceID = source.ID
		lastFailure.Reason = fmt.Sprintf("evidence %s status %q not in allowlist %v",
			source.ID, source.Status, allowed)
	}

	return lastFailure
}

func computePhaseSatisfaction(phaseID string, includeReferences bool, rf RelationFetcher, ef EntityFetcher) (*PhaseSatisfaction, error) {
	closure, err := computePhaseClosure(phaseID, includeReferences, rf)
	if err != nil {
		return nil, err
	}

	report := &PhaseSatisfaction{
		PhaseID: phaseID,
		Items:   make([]SatisfactionItem, 0, len(closure)),
	}

	for _, member := range closure {
		item := evaluateMember(member, phaseID, ef, rf)
		report.Items = append(report.Items, item)

		switch item.Status {
		case SatisfactionSatisfied:
			report.Satisfied++
			report.Total++
		case SatisfactionUnsatisfied:
			report.Total++
		case SatisfactionAdvisory:
			report.AdvisoryCount++
		}
	}

	return report, nil
}

// checkPhaseSatisfaction emits ValidationIssues for every unsatisfied mandatory
// closure member across the targeted phases and returns per-phase reports.
// Advisory members never produce issues but are visible in the reports.
//
// Fetch errors during phase selection or closure computation produce a
// high-severity validation issue rather than being silently skipped, so a
// storage failure cannot masquerade as success.
func checkPhaseSatisfaction(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) ([]ValidationIssue, []PhaseSatisfaction) {
	phases, err := selectPhases(opts, ef)
	if err != nil {
		entity := ""
		if opts.Phase != nil {
			entity = *opts.Phase
		}
		return []ValidationIssue{{
			Check:    "phase_satisfaction",
			Severity: SeverityHigh,
			Entity:   entity,
			Message:  fmt.Sprintf("could not select phases: %v", err),
			Layer:    model.LayerMapping,
		}}, nil
	}

	var issues []ValidationIssue
	reports := make([]PhaseSatisfaction, 0, len(phases))

	for _, phase := range phases {
		report, err := computePhaseSatisfaction(phase.ID, opts.IncludeReferences, rf, ef)
		if err != nil {
			issues = append(issues, ValidationIssue{
				Check:    "phase_satisfaction",
				Severity: SeverityHigh,
				Entity:   phase.ID,
				Message:  fmt.Sprintf("could not compute phase %s satisfaction: %v", phase.ID, err),
				Layer:    model.LayerMapping,
			})
			continue
		}
		reports = append(reports, *report)

		for _, item := range report.Items {
			if item.Status != SatisfactionUnsatisfied {
				continue
			}
			issues = append(issues, ValidationIssue{
				Check:    "phase_satisfaction",
				Severity: SeverityHigh,
				Entity:   item.EntityID,
				Message: fmt.Sprintf("phase %s: %s unsatisfied — %s",
					phase.ID, item.EntityID, item.Reason),
				Layer: model.LayerMapping,
			})
		}
	}

	return issues, reports
}

func selectPhases(opts ValidateOptions, ef EntityFetcher) ([]model.Entity, error) {
	if opts.Phase != nil {
		p, err := ef.Get(*opts.Phase)
		if err != nil {
			return nil, err
		}
		if p.Type != model.EntityTypePhase {
			return nil, fmt.Errorf("entity %q is type %q, not phase", *opts.Phase, p.Type)
		}
		return []model.Entity{p}, nil
	}

	phaseType := model.EntityTypePhase
	activeStatus := model.EntityStatusActive
	execLayer := model.LayerExec
	return ef.List(EntityListFilters{Type: &phaseType, Status: &activeStatus, Layer: &execLayer})
}
