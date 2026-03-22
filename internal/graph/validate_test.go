package graph

import (
	"errors"
	"testing"

	"github.com/taeyeong/spec-graph/internal/model"
)

type mockRelFetcher struct {
	relations map[string][]model.Relation
}

func (m *mockRelFetcher) GetByEntity(entityID string) ([]model.Relation, error) {
	return m.relations[entityID], nil
}

type mockEntFetcher struct {
	entities []model.Entity
	byID     map[string]model.Entity
}

func (m *mockEntFetcher) Get(id string) (model.Entity, error) {
	e, ok := m.byID[id]
	if !ok {
		return model.Entity{}, &model.ErrEntityNotFound{ID: id}
	}
	return e, nil
}

func (m *mockEntFetcher) List(filters EntityListFilters) ([]model.Entity, error) {
	var result []model.Entity
	for _, e := range m.entities {
		if filters.Type != nil && e.Type != *filters.Type {
			continue
		}
		if filters.Status != nil && e.Status != *filters.Status {
			continue
		}
		result = append(result, e)
	}
	return result, nil
}

func TestValidate_PhaseScoped_OrphansFiltered(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "PHS-001", Type: model.EntityTypePhase, Status: model.EntityStatusActive},
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"PHS-001": {ID: "PHS-001", Type: model.EntityTypePhase, Status: model.EntityStatusActive},
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"REQ-002": {ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {{ID: 1, FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn}},
			"PHS-001": {{ID: 1, FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn}},
		},
	}

	phaseID := "PHS-001"
	result, err := Validate(ValidateOptions{Checks: []string{"orphans"}, Phase: &phaseID}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, issue := range result.Issues {
		if issue.Entity == "REQ-002" {
			t.Error("REQ-002 should not appear: it is not in phase scope")
		}
	}
}

func TestValidate_PhaseScoped_CoverageFiltered(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "PHS-001", Type: model.EntityTypePhase, Status: model.EntityStatusActive},
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"PHS-001": {ID: "PHS-001", Type: model.EntityTypePhase, Status: model.EntityStatusActive},
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"REQ-002": {ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {{ID: 1, FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn}},
			"PHS-001": {{ID: 1, FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn}},
		},
	}

	phaseID := "PHS-001"
	result, err := Validate(ValidateOptions{Checks: []string{"coverage"}, Phase: &phaseID}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, issue := range result.Issues {
		if issue.Entity == "REQ-002" {
			t.Error("REQ-002 should not appear: it is not in phase scope")
		}
	}

	foundREQ001 := false
	for _, issue := range result.Issues {
		if issue.Entity == "REQ-001" {
			foundREQ001 = true
		}
	}
	if !foundREQ001 {
		t.Error("expected coverage issue for REQ-001 (in phase scope, no implementation)")
	}
}

func TestValidate_NoPhase_Unchanged(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{}}

	result, err := Validate(ValidateOptions{}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected Valid=false for orphaned entity")
	}
	if result.Summary.TotalIssues != 3 {
		t.Errorf("got TotalIssues=%d; want 3", result.Summary.TotalIssues)
	}
}

func TestCheckGates_UnresolvedQuestion(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "QST-001", Type: model.EntityTypeQuestion, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"QST-001": {ID: "QST-001", Type: model.EntityTypeQuestion, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{}}

	issues, err := checkGates(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues; want 1", len(issues))
	}
	if issues[0].Entity != "QST-001" {
		t.Errorf("entity=%q; want QST-001", issues[0].Entity)
	}
	if issues[0].Check != "gates" {
		t.Errorf("check=%q; want gates", issues[0].Check)
	}
	if issues[0].Severity != SeverityHigh {
		t.Errorf("severity=%q; want high", issues[0].Severity)
	}
}

func TestCheckGates_ResolvedQuestion_NoIssue(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "QST-001", Type: model.EntityTypeQuestion, Status: model.EntityStatusResolved},
		},
		byID: map[string]model.Entity{
			"QST-001": {ID: "QST-001", Type: model.EntityTypeQuestion, Status: model.EntityStatusResolved},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{}}

	issues, err := checkGates(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0 (resolved question)", len(issues))
	}
}

func TestCheckGates_UnmitigatedRisk(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "RSK-001", Type: model.EntityTypeRisk, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"RSK-001": {ID: "RSK-001", Type: model.EntityTypeRisk, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{
		"RSK-001": {},
	}}

	issues, err := checkGates(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues; want 1", len(issues))
	}
	if issues[0].Entity != "RSK-001" {
		t.Errorf("entity=%q; want RSK-001", issues[0].Entity)
	}
	if issues[0].Severity != SeverityHigh {
		t.Errorf("severity=%q; want high", issues[0].Severity)
	}
}

func TestCheckGates_MitigatedRisk_NoIssue(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "RSK-001", Type: model.EntityTypeRisk, Status: model.EntityStatusActive},
			{ID: "DEC-001", Type: model.EntityTypeDecision, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"RSK-001": {ID: "RSK-001", Type: model.EntityTypeRisk, Status: model.EntityStatusActive},
			"DEC-001": {ID: "DEC-001", Type: model.EntityTypeDecision, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{
		"RSK-001": {{ID: 1, FromID: "DEC-001", ToID: "RSK-001", Type: model.RelationMitigates}},
		"DEC-001": {{ID: 1, FromID: "DEC-001", ToID: "RSK-001", Type: model.RelationMitigates}},
	}}

	issues, err := checkGates(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0 (mitigated risk): %+v", len(issues), issues)
	}
}

func TestCheckGates_ResolvedRisk_NoIssue(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "RSK-001", Type: model.EntityTypeRisk, Status: model.EntityStatusResolved},
		},
		byID: map[string]model.Entity{
			"RSK-001": {ID: "RSK-001", Type: model.EntityTypeRisk, Status: model.EntityStatusResolved},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{}}

	issues, err := checkGates(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0 (resolved risk)", len(issues))
	}
}

func TestCheckGates_DraftDecisionDependency(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "DEC-001", Type: model.EntityTypeDecision, Status: model.EntityStatusDraft},
		},
		byID: map[string]model.Entity{
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"DEC-001": {ID: "DEC-001", Type: model.EntityTypeDecision, Status: model.EntityStatusDraft},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{
		"REQ-001": {{ID: 1, FromID: "REQ-001", ToID: "DEC-001", Type: model.RelationDependsOn}},
		"DEC-001": {{ID: 1, FromID: "REQ-001", ToID: "DEC-001", Type: model.RelationDependsOn}},
	}}

	issues, err := checkGates(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues; want 1", len(issues))
	}
	if issues[0].Entity != "REQ-001" {
		t.Errorf("entity=%q; want REQ-001", issues[0].Entity)
	}
	if issues[0].Severity != SeverityHigh {
		t.Errorf("severity=%q; want high", issues[0].Severity)
	}
}

func TestCheckGates_ActiveDecisionDependency_NoIssue(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "DEC-001", Type: model.EntityTypeDecision, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"DEC-001": {ID: "DEC-001", Type: model.EntityTypeDecision, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{
		"REQ-001": {{ID: 1, FromID: "REQ-001", ToID: "DEC-001", Type: model.RelationDependsOn}},
		"DEC-001": {{ID: 1, FromID: "REQ-001", ToID: "DEC-001", Type: model.RelationDependsOn}},
	}}

	issues, err := checkGates(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0 (active decision): %+v", len(issues), issues)
	}
}

func TestCheckGates_DraftDecisionNoDependent_NoIssue(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "DEC-001", Type: model.EntityTypeDecision, Status: model.EntityStatusDraft},
		},
		byID: map[string]model.Entity{
			"DEC-001": {ID: "DEC-001", Type: model.EntityTypeDecision, Status: model.EntityStatusDraft},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{}}

	issues, err := checkGates(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0 (draft decision with no active dependents): %+v", len(issues), issues)
	}
}

func TestCheckGates_UnresolvedAssumption(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "ASM-001", Type: model.EntityTypeAssumption, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"ASM-001": {ID: "ASM-001", Type: model.EntityTypeAssumption, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{
		"REQ-001": {{ID: 1, FromID: "REQ-001", ToID: "ASM-001", Type: model.RelationAssumes}},
		"ASM-001": {{ID: 1, FromID: "REQ-001", ToID: "ASM-001", Type: model.RelationAssumes}},
	}}

	issues, err := checkGates(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, issue := range issues {
		if issue.Entity == "ASM-001" && issue.Check == "gates" && issue.Severity == SeverityMedium {
			found = true
		}
	}
	if !found {
		t.Errorf("expected gate issue for ASM-001 (medium severity), got: %+v", issues)
	}
}

func TestCheckGates_ResolvedAssumption_NoIssue(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "ASM-001", Type: model.EntityTypeAssumption, Status: model.EntityStatusResolved},
		},
		byID: map[string]model.Entity{
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"ASM-001": {ID: "ASM-001", Type: model.EntityTypeAssumption, Status: model.EntityStatusResolved},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{
		"REQ-001": {{ID: 1, FromID: "REQ-001", ToID: "ASM-001", Type: model.RelationAssumes}},
		"ASM-001": {{ID: 1, FromID: "REQ-001", ToID: "ASM-001", Type: model.RelationAssumes}},
	}}

	issues, err := checkGates(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, issue := range issues {
		if issue.Entity == "ASM-001" {
			t.Errorf("unexpected issue for resolved assumption: %+v", issue)
		}
	}
}

func TestCheckGates_CompletionGap(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "PHS-001", Type: model.EntityTypePhase, Status: model.EntityStatusActive},
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"PHS-001": {ID: "PHS-001", Type: model.EntityTypePhase, Status: model.EntityStatusActive},
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{
		"REQ-001": {{ID: 1, FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn}},
		"PHS-001": {{ID: 1, FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn}},
	}}

	issues, err := checkGates(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, issue := range issues {
		if issue.Entity == "REQ-001" && issue.Check == "gates" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected completion gap issue for REQ-001, got: %+v", issues)
	}
}

func TestCheckGates_CompletionGap_AllDelivered_NoIssue(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "PHS-001", Type: model.EntityTypePhase, Status: model.EntityStatusActive},
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"PHS-001": {ID: "PHS-001", Type: model.EntityTypePhase, Status: model.EntityStatusActive},
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"API-001": {ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{
		"REQ-001": {
			{ID: 1, FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn},
			{ID: 2, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements},
		},
		"API-001": {
			{ID: 2, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements},
			{ID: 3, FromID: "API-001", ToID: "PHS-001", Type: model.RelationDeliveredIn},
		},
		"PHS-001": {
			{ID: 1, FromID: "REQ-001", ToID: "PHS-001", Type: model.RelationPlannedIn},
			{ID: 3, FromID: "API-001", ToID: "PHS-001", Type: model.RelationDeliveredIn},
		},
	}}

	issues, err := checkGates(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, issue := range issues {
		if issue.Entity == "REQ-001" {
			t.Errorf("unexpected completion gap issue for REQ-001 (should be delivered): %+v", issue)
		}
	}
}

func TestCheckGates_CompletionGap_QuestionResolved_NoIssue(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "PHS-001", Type: model.EntityTypePhase, Status: model.EntityStatusActive},
			{ID: "QST-001", Type: model.EntityTypeQuestion, Status: model.EntityStatusResolved},
		},
		byID: map[string]model.Entity{
			"PHS-001": {ID: "PHS-001", Type: model.EntityTypePhase, Status: model.EntityStatusActive},
			"QST-001": {ID: "QST-001", Type: model.EntityTypeQuestion, Status: model.EntityStatusResolved},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{
		"QST-001": {{ID: 1, FromID: "QST-001", ToID: "PHS-001", Type: model.RelationPlannedIn}},
		"PHS-001": {{ID: 1, FromID: "QST-001", ToID: "PHS-001", Type: model.RelationPlannedIn}},
	}}

	issues, err := checkGates(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, issue := range issues {
		if issue.Entity == "QST-001" {
			t.Errorf("unexpected issue for resolved question (should count as delivered): %+v", issue)
		}
	}
}

func TestValidateOrphans_NoOrphans(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"API-001": {ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {{ID: 1, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements}},
			"API-001": {{ID: 1, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements}},
		},
	}

	issues, err := checkOrphans(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0", len(issues))
	}
}

func TestValidateOrphans_OneOrphan(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"API-001": {{ID: 1, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements}},
		},
	}

	issues, err := checkOrphans(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues; want 1", len(issues))
	}
	if issues[0].Entity != "REQ-001" {
		t.Errorf("got entity %q; want %q", issues[0].Entity, "REQ-001")
	}
	if issues[0].Check != "orphans" {
		t.Errorf("got check %q; want %q", issues[0].Check, "orphans")
	}
	if issues[0].Severity != SeverityMedium {
		t.Errorf("got severity %q; want %q", issues[0].Severity, SeverityMedium)
	}
}

func TestValidateOrphans_MultipleOrphans(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusDraft},
			{ID: "DEC-001", Type: model.EntityTypeDecision, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{}}

	issues, err := checkOrphans(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 3 {
		t.Errorf("got %d issues; want 3", len(issues))
	}
}

func TestValidateOrphans_IgnoreDeletedEntities(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusDeleted},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{}}

	issues, err := checkOrphans(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0 (deleted entities should be skipped)", len(issues))
	}
}

func TestValidateOrphans_IgnoreDeprecatedEntities(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusDeprecated},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{}}

	issues, err := checkOrphans(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0 (deprecated entities should be skipped)", len(issues))
	}
}

func TestValidateOrphans_EmptyGraph(t *testing.T) {
	ef := &mockEntFetcher{entities: []model.Entity{}}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{}}

	issues, err := checkOrphans(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0", len(issues))
	}
}

func TestValidate(t *testing.T) {
	t.Run("AllChecks", func(t *testing.T) {
		ef := &mockEntFetcher{
			entities: []model.Entity{
				{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			},
		}
		rf := &mockRelFetcher{relations: map[string][]model.Relation{}}

		result, err := Validate(ValidateOptions{}, rf, ef)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Valid {
			t.Error("expected Valid=false for orphaned entity")
		}
		if result.Summary.TotalIssues != 3 {
			t.Errorf("got TotalIssues=%d; want 3", result.Summary.TotalIssues)
		}
		if result.Summary.BySeverity[SeverityMedium] != 1 {
			t.Errorf("got BySeverity[medium]=%d; want 1", result.Summary.BySeverity[SeverityMedium])
		}
		if result.Summary.BySeverity[SeverityHigh] != 2 {
			t.Errorf("got BySeverity[high]=%d; want 2", result.Summary.BySeverity[SeverityHigh])
		}
	})

	t.Run("SpecificCheck", func(t *testing.T) {
		ef := &mockEntFetcher{entities: []model.Entity{}}
		rf := &mockRelFetcher{relations: map[string][]model.Relation{}}

		result, err := Validate(ValidateOptions{Checks: []string{"orphans"}}, rf, ef)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Valid {
			t.Error("expected Valid=true for empty graph")
		}
		if result.Summary.TotalIssues != 0 {
			t.Errorf("got TotalIssues=%d; want 0", result.Summary.TotalIssues)
		}
	})

	t.Run("InvalidCheck", func(t *testing.T) {
		ef := &mockEntFetcher{entities: []model.Entity{}}
		rf := &mockRelFetcher{relations: map[string][]model.Relation{}}

		_, err := Validate(ValidateOptions{Checks: []string{"nonexistent"}}, rf, ef)
		if err == nil {
			t.Fatal("expected error for unknown check")
		}
		var target *model.ErrInvalidInput
		if !errors.As(err, &target) {
			t.Errorf("expected ErrInvalidInput; got %T: %v", err, err)
		}
	})
}

func TestValidateCoverage_AllCovered(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "ACT-001", Type: model.EntityTypeCriterion, Status: model.EntityStatusActive},
			{ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {
				{ID: 1, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements},
				{ID: 2, FromID: "REQ-001", ToID: "ACT-001", Type: model.RelationHasCriterion},
			},
			"ACT-001": {
				{ID: 2, FromID: "REQ-001", ToID: "ACT-001", Type: model.RelationHasCriterion},
				{ID: 3, FromID: "TST-001", ToID: "ACT-001", Type: model.RelationVerifies},
			},
			"API-001": {
				{ID: 1, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements},
				{ID: 4, FromID: "API-001", ToID: "STT-001", Type: model.RelationTriggers},
				{ID: 5, FromID: "TST-002", ToID: "API-001", Type: model.RelationVerifies},
			},
		},
	}

	issues, err := checkCoverage(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0: %+v", len(issues), issues)
	}
}

func TestValidateCoverage_RequirementMissingImplements(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {
				{ID: 1, FromID: "REQ-001", ToID: "ACT-001", Type: model.RelationHasCriterion},
			},
		},
	}

	issues, err := checkCoverage(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues; want 1: %+v", len(issues), issues)
	}
	if issues[0].Message != "requirement has no implementation" {
		t.Errorf("got message %q; want %q", issues[0].Message, "requirement has no implementation")
	}
	if issues[0].Severity != SeverityHigh {
		t.Errorf("got severity %q; want %q", issues[0].Severity, SeverityHigh)
	}
}

func TestValidateCoverage_RequirementMissingCriterion(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {
				{ID: 1, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements},
			},
		},
	}

	issues, err := checkCoverage(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues; want 1: %+v", len(issues), issues)
	}
	if issues[0].Message != "requirement has no acceptance criterion" {
		t.Errorf("got message %q; want %q", issues[0].Message, "requirement has no acceptance criterion")
	}
}

func TestValidateCoverage_CriterionMissingVerifies(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "ACT-001", Type: model.EntityTypeCriterion, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"ACT-001": {
				{ID: 1, FromID: "REQ-001", ToID: "ACT-001", Type: model.RelationHasCriterion},
			},
		},
	}

	issues, err := checkCoverage(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues; want 1: %+v", len(issues), issues)
	}
	if issues[0].Message != "criterion has no verification" {
		t.Errorf("got message %q; want %q", issues[0].Message, "criterion has no verification")
	}
}

func TestValidateCoverage_TriggeringInterfaceMissingTest(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "API-005", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"API-005": {
				{ID: 1, FromID: "API-005", ToID: "STT-001", Type: model.RelationTriggers},
			},
		},
	}

	issues, err := checkCoverage(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues; want 1: %+v", len(issues), issues)
	}
	if issues[0].Message != "interface triggers state but has no verifying test" {
		t.Errorf("got message %q; want %q", issues[0].Message, "interface triggers state but has no verifying test")
	}
	if issues[0].Entity != "API-005" {
		t.Errorf("got entity %q; want %q", issues[0].Entity, "API-005")
	}
}

func TestValidateCoverage_IgnoreNonActiveEntities(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusDraft},
			{ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusDeprecated},
			{ID: "ACT-001", Type: model.EntityTypeCriterion, Status: model.EntityStatusResolved},
			{ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusDeleted},
		},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{}}

	issues, err := checkCoverage(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0 (non-active entities should be skipped): %+v", len(issues), issues)
	}
}

func TestValidateCoverage_MultipleIssues(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "ACT-001", Type: model.EntityTypeCriterion, Status: model.EntityStatusActive},
			{ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"API-001": {
				{ID: 1, FromID: "API-001", ToID: "STT-001", Type: model.RelationTriggers},
			},
		},
	}

	issues, err := checkCoverage(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) < 4 {
		t.Errorf("got %d issues; want at least 4: %+v", len(issues), issues)
	}
}

func TestValidateCoverage_EmptyGraph(t *testing.T) {
	ef := &mockEntFetcher{entities: []model.Entity{}}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{}}

	issues, err := checkCoverage(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0", len(issues))
	}
}

func TestValidateInvalidEdges_AllValid(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"API-001": {ID: "API-001", Type: model.EntityTypeInterface},
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"API-001": {{ID: 1, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements}},
			"REQ-001": {{ID: 1, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements}},
		},
	}

	issues, err := checkInvalidEdges(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0: %+v", len(issues), issues)
	}
}

func TestValidateInvalidEdges_OneInvalid(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "DEC-001", Type: model.EntityTypeDecision, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement},
			"DEC-001": {ID: "DEC-001", Type: model.EntityTypeDecision},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {{ID: 1, FromID: "REQ-001", ToID: "DEC-001", Type: model.RelationImplements}},
			"DEC-001": {{ID: 1, FromID: "REQ-001", ToID: "DEC-001", Type: model.RelationImplements}},
		},
	}

	issues, err := checkInvalidEdges(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues; want 1: %+v", len(issues), issues)
	}
	if issues[0].Check != "invalid_edges" {
		t.Errorf("got check %q; want %q", issues[0].Check, "invalid_edges")
	}
	if issues[0].Severity != SeverityHigh {
		t.Errorf("got severity %q; want %q", issues[0].Severity, SeverityHigh)
	}
	if issues[0].Entity != "REQ-001" {
		t.Errorf("got entity %q; want %q", issues[0].Entity, "REQ-001")
	}
}

func TestValidateInvalidEdges_MultipleInvalid(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "DEC-001", Type: model.EntityTypeDecision, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement},
			"REQ-002": {ID: "REQ-002", Type: model.EntityTypeRequirement},
			"DEC-001": {ID: "DEC-001", Type: model.EntityTypeDecision},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {{ID: 1, FromID: "REQ-001", ToID: "DEC-001", Type: model.RelationImplements}},
			"REQ-002": {{ID: 2, FromID: "REQ-002", ToID: "DEC-001", Type: model.RelationImplements}},
			"DEC-001": {
				{ID: 1, FromID: "REQ-001", ToID: "DEC-001", Type: model.RelationImplements},
				{ID: 2, FromID: "REQ-002", ToID: "DEC-001", Type: model.RelationImplements},
			},
		},
	}

	issues, err := checkInvalidEdges(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("got %d issues; want 2: %+v", len(issues), issues)
	}
}

func TestValidateInvalidEdges_EmptyGraph(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{},
		byID:     map[string]model.Entity{},
	}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{}}

	issues, err := checkInvalidEdges(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0", len(issues))
	}
}

func TestValidateInvalidEdges_SupersedesSameType(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement},
			"REQ-002": {ID: "REQ-002", Type: model.EntityTypeRequirement},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {{ID: 1, FromID: "REQ-001", ToID: "REQ-002", Type: model.RelationSupersedes}},
			"REQ-002": {{ID: 1, FromID: "REQ-001", ToID: "REQ-002", Type: model.RelationSupersedes}},
		},
	}

	issues, err := checkInvalidEdges(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0: %+v", len(issues), issues)
	}
}

func TestValidateInvalidEdges_SupersedesDifferentType(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "DEC-001", Type: model.EntityTypeDecision, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement},
			"DEC-001": {ID: "DEC-001", Type: model.EntityTypeDecision},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {{ID: 1, FromID: "REQ-001", ToID: "DEC-001", Type: model.RelationSupersedes}},
			"DEC-001": {{ID: 1, FromID: "REQ-001", ToID: "DEC-001", Type: model.RelationSupersedes}},
		},
	}

	issues, err := checkInvalidEdges(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues; want 1: %+v", len(issues), issues)
	}
	if issues[0].Entity != "REQ-001" {
		t.Errorf("got entity %q; want %q", issues[0].Entity, "REQ-001")
	}
}

func TestValidateSupersededRefs_NoIssues(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"API-001": {ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {{ID: 1, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements}},
			"API-001": {{ID: 1, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements}},
		},
	}

	issues, err := checkSupersededRefs(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0: %+v", len(issues), issues)
	}
}

func TestValidateSupersededRefs_StaleReference(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "API-005", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"REQ-002": {ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"API-005": {ID: "API-005", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {
				{ID: 1, FromID: "REQ-002", ToID: "REQ-001", Type: model.RelationSupersedes},
				{ID: 2, FromID: "API-005", ToID: "REQ-001", Type: model.RelationImplements},
			},
			"REQ-002": {
				{ID: 1, FromID: "REQ-002", ToID: "REQ-001", Type: model.RelationSupersedes},
			},
			"API-005": {
				{ID: 2, FromID: "API-005", ToID: "REQ-001", Type: model.RelationImplements},
			},
		},
	}

	issues, err := checkSupersededRefs(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues; want 1: %+v", len(issues), issues)
	}
	if issues[0].Entity != "API-005" {
		t.Errorf("got entity %q; want %q", issues[0].Entity, "API-005")
	}
	if issues[0].Severity != SeverityHigh {
		t.Errorf("got severity %q; want %q", issues[0].Severity, SeverityHigh)
	}
	if issues[0].Check != "superseded_refs" {
		t.Errorf("got check %q; want %q", issues[0].Check, "superseded_refs")
	}
}

func TestValidateSupersededRefs_CleanSupersede(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"REQ-002": {ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {
				{ID: 1, FromID: "REQ-002", ToID: "REQ-001", Type: model.RelationSupersedes},
			},
			"REQ-002": {
				{ID: 1, FromID: "REQ-002", ToID: "REQ-001", Type: model.RelationSupersedes},
			},
		},
	}

	issues, err := checkSupersededRefs(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0: %+v", len(issues), issues)
	}
}

func TestValidateSupersededRefs_MultipleStaleRefs(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
			{ID: "TST-001", Type: model.EntityTypeTest, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"REQ-002": {ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"API-001": {ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
			"TST-001": {ID: "TST-001", Type: model.EntityTypeTest, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {
				{ID: 1, FromID: "REQ-002", ToID: "REQ-001", Type: model.RelationSupersedes},
				{ID: 2, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements},
				{ID: 3, FromID: "TST-001", ToID: "REQ-001", Type: model.RelationVerifies},
			},
			"REQ-002": {
				{ID: 1, FromID: "REQ-002", ToID: "REQ-001", Type: model.RelationSupersedes},
			},
			"API-001": {
				{ID: 2, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements},
			},
			"TST-001": {
				{ID: 3, FromID: "TST-001", ToID: "REQ-001", Type: model.RelationVerifies},
			},
		},
	}

	issues, err := checkSupersededRefs(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("got %d issues; want 2: %+v", len(issues), issues)
	}
}

func TestValidateSupersededRefs_SupersedesChain(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "REQ-003", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
		},
		byID: map[string]model.Entity{
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"REQ-002": {ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"REQ-003": {ID: "REQ-003", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"API-001": {ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusActive},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {
				{ID: 1, FromID: "REQ-002", ToID: "REQ-001", Type: model.RelationSupersedes},
				{ID: 3, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements},
			},
			"REQ-002": {
				{ID: 1, FromID: "REQ-002", ToID: "REQ-001", Type: model.RelationSupersedes},
				{ID: 2, FromID: "REQ-003", ToID: "REQ-002", Type: model.RelationSupersedes},
			},
			"REQ-003": {
				{ID: 2, FromID: "REQ-003", ToID: "REQ-002", Type: model.RelationSupersedes},
			},
			"API-001": {
				{ID: 3, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements},
			},
		},
	}

	issues, err := checkSupersededRefs(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues; want 1: %+v", len(issues), issues)
	}
	if issues[0].Entity != "API-001" {
		t.Errorf("got entity %q; want %q", issues[0].Entity, "API-001")
	}
}

func TestValidate_GatesCheckRegistered(t *testing.T) {
	ef := &mockEntFetcher{entities: []model.Entity{}, byID: map[string]model.Entity{}}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{}}

	result, err := Validate(ValidateOptions{Checks: []string{"gates"}}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: gates check should be recognized, got %v", err)
	}
	if !result.Valid {
		t.Error("expected Valid=true for empty graph with gates check")
	}
	if result.Summary.TotalIssues != 0 {
		t.Errorf("got TotalIssues=%d; want 0", result.Summary.TotalIssues)
	}
}

func TestValidate_DefaultChecksIncludeGates(t *testing.T) {
	ef := &mockEntFetcher{entities: []model.Entity{}, byID: map[string]model.Entity{}}
	rf := &mockRelFetcher{relations: map[string][]model.Relation{}}

	result, err := Validate(ValidateOptions{}, rf, ef)
	if err != nil {
		t.Fatalf("unexpected error: default checks should include gates without error, got %v", err)
	}
	if !result.Valid {
		t.Error("expected Valid=true for empty graph")
	}
}

func TestValidateSupersededRefs_DeprecatedEntityReferencingSuperseded(t *testing.T) {
	ef := &mockEntFetcher{
		entities: []model.Entity{
			{ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			{ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusDeprecated},
		},
		byID: map[string]model.Entity{
			"REQ-001": {ID: "REQ-001", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"REQ-002": {ID: "REQ-002", Type: model.EntityTypeRequirement, Status: model.EntityStatusActive},
			"API-001": {ID: "API-001", Type: model.EntityTypeInterface, Status: model.EntityStatusDeprecated},
		},
	}
	rf := &mockRelFetcher{
		relations: map[string][]model.Relation{
			"REQ-001": {
				{ID: 1, FromID: "REQ-002", ToID: "REQ-001", Type: model.RelationSupersedes},
				{ID: 2, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements},
			},
			"REQ-002": {
				{ID: 1, FromID: "REQ-002", ToID: "REQ-001", Type: model.RelationSupersedes},
			},
			"API-001": {
				{ID: 2, FromID: "API-001", ToID: "REQ-001", Type: model.RelationImplements},
			},
		},
	}

	issues, err := checkSupersededRefs(ef, rf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues; want 0 (deprecated entity should not be flagged): %+v", len(issues), issues)
	}
}
