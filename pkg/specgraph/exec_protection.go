package specgraph

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/tyeongkim/spec-graph/internal/gate"
	"github.com/tyeongkim/spec-graph/internal/model"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
	"github.com/tyeongkim/spec-graph/internal/validate"
)

// A task may still fill qa[].evidence while frozen, because that field is how
// resolution records its proof.
func frozenFieldChanges(stored model.Entity, candidate *spectoml.EntityFile) ([]string, error) {
	if !model.IsExecType(stored.Type) || !model.ExecLifecycleStarted(stored.Status) {
		return nil, nil
	}

	var changed []string
	if stored.Title != candidate.Title {
		changed = append(changed, "title")
	}
	if stored.Description != candidate.Description {
		changed = append(changed, "description")
	}

	var storedMetadata map[string]any
	if len(stored.Metadata) > 0 {
		if err := json.Unmarshal(stored.Metadata, &storedMetadata); err != nil {
			return nil, newError(CodeRuntime, fmt.Sprintf("decode stored metadata of %q", stored.ID), err)
		}
	}
	candidateMetadata, err := roundTripMetadata(candidate.Metadata)
	if err != nil {
		return nil, newError(CodeRuntime, fmt.Sprintf("encode candidate metadata of %q", stored.ID), err)
	}
	if stored.Type == model.EntityTypeTask {
		stripTaskEvidence(storedMetadata)
		stripTaskEvidence(candidateMetadata)
	}
	if !metadataEqual(storedMetadata, candidateMetadata) {
		changed = append(changed, "metadata")
	}
	return changed, nil
}

func roundTripMetadata(metadata map[string]any) (map[string]any, error) {
	if len(metadata) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func metadataEqual(a, b map[string]any) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

func stripTaskEvidence(metadata map[string]any) {
	items, ok := metadata["qa"].([]any)
	if !ok {
		return
	}
	for _, item := range items {
		if qa, ok := item.(map[string]any); ok {
			delete(qa, "evidence")
		}
	}
}

func frozenFieldReport(stored model.Entity, toStatus model.EntityStatus, changed []string) *gate.Report {
	report := &gate.Report{
		Blocked:    true,
		EntityID:   stored.ID,
		EntityType: stored.Type,
		FromStatus: stored.Status,
		ToStatus:   toStatus,
	}
	for _, field := range changed {
		report.BlockingIssues = append(report.BlockingIssues, validate.ValidationIssue{
			Check:    "frozen_fields",
			Severity: validate.SeverityHigh,
			Entity:   stored.ID,
			Message:  fmt.Sprintf("%s %s is %s; %s cannot change without --force --reason", stored.Type, stored.ID, stored.Status, field),
			Layer:    model.LayerExec,
		})
	}
	return report
}

func mergeGateReports(base, next *gate.Report) *gate.Report {
	if base == nil {
		return next
	}
	base.BlockingIssues = append(base.BlockingIssues, next.BlockingIssues...)
	base.Warnings = append(base.Warnings, next.Warnings...)
	base.ToStatus = next.ToStatus
	base.StructuralBlocked = base.StructuralBlocked || next.StructuralBlocked
	return base
}

// A target that no longer exists is a dangling edge, and removing it is cleanup
// rather than a scope change.
func (t *txn) checkRelationDeletion(rt model.RelationType, from model.Entity, targetID string) error {
	to, err := (&stagedEntityFetcher{tx: t}).Get(targetID)
	if err != nil {
		var missing *model.ErrEntityNotFound
		if errors.As(err, &missing) {
			return nil
		}
		return newError(CodeRuntime, fmt.Sprintf("lookup target entity %q", targetID), err)
	}
	return checkRelationMutation("delete", rt, from, to)
}

func checkRelationMutation(verb string, rt model.RelationType, from, to model.Entity) error {
	refuse := func(entity model.Entity, why string) error {
		return newError(
			CodeInvalidInput,
			fmt.Sprintf("cannot %s %q relation: %s %q is %s and %s", verb, rt, entity.Type, entity.ID, entity.Status, why),
			nil,
		)
	}

	switch rt {
	case model.RelationBelongsTo:
		if model.ExecLifecycleStarted(from.Status) {
			return refuse(from, "its parent is fixed")
		}
		if from.Type == model.EntityTypeTask && model.ExecLifecycleStarted(to.Status) {
			return refuse(to, "its task set is fixed")
		}
	case model.RelationTaskDependsOn:
		if model.ExecLifecycleStarted(from.Status) {
			return refuse(from, "its prerequisites are fixed")
		}
	case model.RelationPrecedes, model.RelationBlocks:
		if model.ExecLifecycleStarted(from.Status) {
			return refuse(from, "its ordering is fixed")
		}
		if model.ExecLifecycleStarted(to.Status) {
			return refuse(to, "its ordering is fixed")
		}
	case model.RelationCovers:
		if model.ExecLifecycleStarted(from.Status) {
			return refuse(from, "its scope is fixed")
		}
	case model.RelationDelivers:
		if from.Status == model.EntityStatusResolved || from.Status == model.EntityStatusDeprecated {
			return refuse(from, "its delivery record is fixed")
		}
	}
	return nil
}
