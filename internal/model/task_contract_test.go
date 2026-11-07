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
