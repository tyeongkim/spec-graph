package model

import (
	"encoding/json"
	"testing"
)

func TestDecodeTaskContractRejectsQABoundaries(t *testing.T) {
	metadata := func(qa string) json.RawMessage {
		return json.RawMessage(`{"order":1,"instructions":["work"],"acceptance":["done"],"must_not":[],"references":[],"qa":` + qa + `}`)
	}
	const validQA = `[{"command":"go test ./...","expected":"exit 0","evidence":""}]`

	tests := []struct {
		name     string
		status   EntityStatus
		metadata json.RawMessage
	}{
		{name: "empty QA", status: EntityStatusDraft, metadata: metadata(`[]`)},
		{name: "blank QA command", status: EntityStatusDraft, metadata: metadata(`[{"command":" \t","expected":"exit 0","evidence":""}]`)},
		{name: "blank QA expected value", status: EntityStatusDraft, metadata: metadata(`[{"command":"go test ./...","expected":" ","evidence":""}]`)},
		{name: "missing QA evidence", status: EntityStatusDraft, metadata: metadata(`[{"command":"go test ./...","expected":"exit 0"}]`)},
		{name: "evidence before resolution", status: EntityStatusActive, metadata: metadata(`[{"command":"go test ./...","expected":"exit 0","evidence":"qa.log"}]`)},
		{name: "empty evidence when resolved", status: EntityStatusResolved, metadata: metadata(validQA)},
		{name: "multiple top-level JSON values", status: EntityStatusDraft, metadata: json.RawMessage(string(metadata(validQA)) + ` {}`)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodeTaskContract(test.metadata, test.status); err == nil {
				t.Fatal("expected task contract rejection")
			}
		})
	}
}

func TestValidateTaskTransitionMatrix(t *testing.T) {
	statuses := []EntityStatus{
		EntityStatusDraft,
		EntityStatusActive,
		EntityStatusResolved,
		EntityStatusDeprecated,
		EntityStatusDeleted,
	}
	transitions := []struct {
		from    EntityStatus
		allowed map[EntityStatus]bool
	}{
		{from: EntityStatusDraft, allowed: map[EntityStatus]bool{EntityStatusActive: true, EntityStatusDeprecated: true}},
		{from: EntityStatusActive, allowed: map[EntityStatus]bool{EntityStatusResolved: true, EntityStatusDeprecated: true}},
		{from: EntityStatusResolved, allowed: map[EntityStatus]bool{}},
		{from: EntityStatusDeprecated, allowed: map[EntityStatus]bool{}},
		{from: EntityStatusDeleted, allowed: map[EntityStatus]bool{}},
	}

	for _, transition := range transitions {
		for _, to := range statuses {
			t.Run(string(transition.from)+" to "+string(to), func(t *testing.T) {
				_, wantAllowed := transition.allowed[to]
				if gotAllowed := ValidateTaskTransition(transition.from, to) == nil; gotAllowed != wantAllowed {
					t.Errorf("transition allowed = %t; want %t", gotAllowed, wantAllowed)
				}
			})
		}
	}
}
