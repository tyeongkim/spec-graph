package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

type updateRPCResult struct {
	Entity struct {
		Status model.EntityStatus `json:"status"`
	} `json:"entity"`
	Outcome    string          `json:"outcome"`
	Blocked    bool            `json:"blocked"`
	GateReport json.RawMessage `json:"gate_report"`
}

type deleteRPCResult struct {
	Deleted string `json:"deleted"`
}

type relationRPCResult struct {
	Relation model.Relation `json:"relation"`
}

type relationListRPCResult struct {
	Relations []model.Relation `json:"relations"`
	Count     int              `json:"count"`
}

func callEntityUpdate(t *testing.T, dispatcher *Dispatcher, params string) updateRPCResult {
	t.Helper()
	request := `{"jsonrpc":"2.0","id":1,"method":"entity.update","params":` + params + `}`
	encoded, notification := dispatcher.Handle(context.Background(), []byte(request))
	if notification {
		t.Fatal("entity.update returned a notification")
	}
	var envelope response
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if envelope.Error != nil {
		t.Fatalf("entity.update error: %+v", envelope.Error)
	}
	var result updateRPCResult
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatalf("unmarshal update result: %v", err)
	}
	return result
}

func TestEntityUpdateRPCDistinguishesForcedApplyFromBlocked(t *testing.T) {
	t.Run("completion force is applied", func(t *testing.T) {
		engine := newTestEngine(t)
		ctx := context.Background()
		if _, err := engine.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type: "phase", ID: "PHS-001", Title: "Phase", Status: "active",
		}); err != nil {
			t.Fatalf("create phase: %v", err)
		}
		if _, err := engine.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type: "question", ID: "QST-001", Title: "Question", Status: "active",
		}); err != nil {
			t.Fatalf("create question: %v", err)
		}
		if _, err := engine.AddRelation(ctx, specgraph.AddRelationRequest{
			From: "PHS-001", To: "QST-001", Type: "covers",
		}); err != nil {
			t.Fatalf("add covers: %v", err)
		}

		result := callEntityUpdate(t, NewDispatcher(engine),
			`{"id":"PHS-001","status":"resolved","force":true,"reason":"Accept question"}`)
		if result.Outcome != "applied_with_force" || result.Blocked {
			t.Fatalf("outcome = %q, blocked = %v", result.Outcome, result.Blocked)
		}
		if result.Entity.Status != model.EntityStatusResolved {
			t.Fatalf("status = %q; want resolved", result.Entity.Status)
		}
		if len(result.GateReport) == 0 {
			t.Fatal("forced update omitted gate report")
		}
	})

	t.Run("structural force is blocked", func(t *testing.T) {
		engine := newTestEngine(t)
		if _, err := engine.CreateEntity(context.Background(), specgraph.CreateEntityRequest{
			Type: "phase", ID: "PHS-001", Title: "Resolved phase", Status: "resolved",
		}); err != nil {
			t.Fatalf("create phase: %v", err)
		}

		result := callEntityUpdate(t, NewDispatcher(engine),
			`{"id":"PHS-001","status":"active","force":true,"reason":"Attempt reopen"}`)
		if result.Outcome != "blocked" || !result.Blocked {
			t.Fatalf("outcome = %q, blocked = %v", result.Outcome, result.Blocked)
		}
		if result.Entity.Status != model.EntityStatusResolved {
			t.Fatalf("status = %q; want resolved", result.Entity.Status)
		}
	})
}

func TestEntityDeleteTransportContracts(t *testing.T) {
	t.Run("deletes an independent entity and returns its ID (catches skipping DeleteEntity)", func(t *testing.T) {
		engine := newTestEngine(t)
		ctx := context.Background()
		if _, err := engine.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type: "requirement", ID: "REQ-001", Title: "Disposable requirement",
		}); err != nil {
			t.Fatalf("create requirement: %v", err)
		}

		result := decodeRPCResult[deleteRPCResult](t, callRPC(t, NewDispatcher(engine), "entity.delete", map[string]any{
			"id": "REQ-001",
		}))
		if result.Deleted != "REQ-001" {
			t.Errorf("deleted = %q; want REQ-001", result.Deleted)
		}
		if _, err := engine.GetEntity(ctx, "REQ-001"); !specgraph.IsNotFound(err) {
			t.Errorf("deleted entity lookup error = %v; want not found", err)
		}
	})

	t.Run("returns invalid params without mutating a referenced entity (catches delete-before-validation)", func(t *testing.T) {
		engine := newTestEngine(t)
		ctx := context.Background()
		for _, entity := range []specgraph.CreateEntityRequest{
			{Type: "requirement", ID: "REQ-001", Title: "Referenced requirement"},
			{Type: "interface", ID: "API-001", Title: "Referencing interface"},
		} {
			if _, err := engine.CreateEntity(ctx, entity); err != nil {
				t.Fatalf("create %s: %v", entity.ID, err)
			}
		}
		if _, err := engine.AddRelation(ctx, specgraph.AddRelationRequest{
			From: "API-001", To: "REQ-001", Type: "implements",
		}); err != nil {
			t.Fatalf("add relation: %v", err)
		}

		envelope := callRPC(t, NewDispatcher(engine), "entity.delete", map[string]any{"id": "REQ-001"})
		if envelope.Error == nil {
			t.Fatal("entity.delete error = nil; want referenced entity deletion rejected")
		}
		if envelope.Error.Code != codeInvalidParams {
			t.Errorf("error code = %d; want %d", envelope.Error.Code, codeInvalidParams)
		}
		dataJSON, err := json.Marshal(envelope.Error.Data)
		if err != nil {
			t.Fatalf("marshal error data: %v", err)
		}
		var data errorData
		if err := json.Unmarshal(dataJSON, &data); err != nil {
			t.Fatalf("unmarshal error data: %v", err)
		}
		if data.Code != string(specgraph.CodeInvalidInput) {
			t.Errorf("error data code = %q; want %q", data.Code, specgraph.CodeInvalidInput)
		}

		entity, err := engine.GetEntity(ctx, "REQ-001")
		if err != nil {
			t.Fatalf("get referenced entity: %v", err)
		}
		if entity.Title != "Referenced requirement" {
			t.Errorf("persisted title = %q; want Referenced requirement", entity.Title)
		}
		relations, count, err := engine.ListRelations(ctx, specgraph.ListRelationsRequest{From: "API-001"})
		if err != nil {
			t.Fatalf("list persisted relations: %v", err)
		}
		if count != 1 || len(relations) != 1 || relations[0].ToID != "REQ-001" {
			t.Errorf("persisted relations = %+v, count = %d; want API-001 -> REQ-001", relations, count)
		}
	})
}

func TestRelationTransportContracts(t *testing.T) {
	t.Run("persists and returns non-default weight and metadata (catches dropping either request field)", func(t *testing.T) {
		engine := newTestEngine(t)
		ctx := context.Background()
		for _, entity := range []specgraph.CreateEntityRequest{
			{Type: "requirement", ID: "REQ-001", Title: "Requirement"},
			{Type: "interface", ID: "API-001", Title: "Interface"},
		} {
			if _, err := engine.CreateEntity(ctx, entity); err != nil {
				t.Fatalf("create %s: %v", entity.ID, err)
			}
		}

		result := decodeRPCResult[relationRPCResult](t, callRPC(t, NewDispatcher(engine), "relation.add", map[string]any{
			"from": "API-001", "to": "REQ-001", "type": "implements", "weight": 0.42,
			"metadata": map[string]any{"source": "rpc", "reviewed": true},
		}))
		if result.Relation.FromID != "API-001" || result.Relation.ToID != "REQ-001" || result.Relation.Type != "implements" {
			t.Errorf("response relation = %+v; want API-001 -> REQ-001 implements", result.Relation)
		}
		if result.Relation.Layer != "arch" {
			t.Errorf("response layer = %q; want arch", result.Relation.Layer)
		}
		if result.Relation.Weight != 0.42 {
			t.Errorf("response weight = %v; want 0.42", result.Relation.Weight)
		}
		var responseMetadata map[string]any
		if err := json.Unmarshal(result.Relation.Metadata, &responseMetadata); err != nil {
			t.Fatalf("unmarshal response metadata: %v", err)
		}
		if responseMetadata["source"] != "rpc" || responseMetadata["reviewed"] != true {
			t.Errorf("response metadata = %v; want source=rpc, reviewed=true", responseMetadata)
		}

		persisted, count, err := engine.ListRelations(ctx, specgraph.ListRelationsRequest{
			From: "API-001", To: "REQ-001", Type: "implements",
		})
		if err != nil {
			t.Fatalf("list persisted relation: %v", err)
		}
		if count != 1 || len(persisted) != 1 {
			t.Fatalf("persisted relations = %+v, count = %d; want one", persisted, count)
		}
		if persisted[0].Weight != 0.42 {
			t.Errorf("persisted weight = %v; want 0.42", persisted[0].Weight)
		}
		var persistedMetadata map[string]any
		if err := json.Unmarshal(persisted[0].Metadata, &persistedMetadata); err != nil {
			t.Fatalf("unmarshal persisted metadata: %v", err)
		}
		if persistedMetadata["source"] != "rpc" || persistedMetadata["reviewed"] != true {
			t.Errorf("persisted metadata = %v; want source=rpc, reviewed=true", persistedMetadata)
		}
	})

	t.Run("forwards endpoint type and layer filters (catches omitting a list filter)", func(t *testing.T) {
		engine := newTestEngine(t)
		ctx := context.Background()
		for _, entity := range []specgraph.CreateEntityRequest{
			{Type: "requirement", ID: "REQ-001", Title: "First requirement"},
			{Type: "requirement", ID: "REQ-002", Title: "Second requirement"},
			{Type: "interface", ID: "API-001", Title: "First interface"},
			{Type: "interface", ID: "API-002", Title: "Second interface"},
			{Type: "plan", ID: "PLN-001", Title: "Plan"},
			{Type: "phase", ID: "PHS-001", Title: "Phase"},
		} {
			if _, err := engine.CreateEntity(ctx, entity); err != nil {
				t.Fatalf("create %s: %v", entity.ID, err)
			}
		}
		for _, relation := range []specgraph.AddRelationRequest{
			{From: "API-001", To: "REQ-001", Type: "implements"},
			{From: "API-002", To: "REQ-001", Type: "implements"},
			{From: "API-001", To: "REQ-002", Type: "implements"},
			{From: "API-001", To: "REQ-001", Type: "references"},
			{From: "PHS-001", To: "PLN-001", Type: "belongs_to"},
			{From: "PHS-001", To: "REQ-002", Type: "covers"},
		} {
			if _, err := engine.AddRelation(ctx, relation); err != nil {
				t.Fatalf("add %s %s -> %s: %v", relation.Type, relation.From, relation.To, err)
			}
		}

		dispatcher := NewDispatcher(engine)
		exact := decodeRPCResult[relationListRPCResult](t, callRPC(t, dispatcher, "relation.list", map[string]any{
			"from": "API-001", "to": "REQ-001", "type": "implements", "layer": "arch",
		}))
		if exact.Count != 1 || len(exact.Relations) != 1 {
			t.Fatalf("exact relation response = %+v; want one relation", exact)
		}
		if relation := exact.Relations[0]; relation.FromID != "API-001" || relation.ToID != "REQ-001" || relation.Type != "implements" || relation.Layer != "arch" {
			t.Errorf("exact relation = %+v; want API-001 -> REQ-001 implements in arch", relation)
		}

		mapping := decodeRPCResult[relationListRPCResult](t, callRPC(t, dispatcher, "relation.list", map[string]any{
			"from": "PHS-001", "layer": "mapping",
		}))
		if mapping.Count != 1 || len(mapping.Relations) != 1 {
			t.Fatalf("mapping relation response = %+v; want one relation", mapping)
		}
		if relation := mapping.Relations[0]; relation.FromID != "PHS-001" || relation.ToID != "REQ-002" || relation.Type != "covers" || relation.Layer != "mapping" {
			t.Errorf("mapping relation = %+v; want PHS-001 -> REQ-002 covers in mapping", relation)
		}
	})

	t.Run("deletes only the requested relation and returns its key (catches skipping DeleteRelation)", func(t *testing.T) {
		engine := newTestEngine(t)
		ctx := context.Background()
		for _, entity := range []specgraph.CreateEntityRequest{
			{Type: "requirement", ID: "REQ-001", Title: "Requirement"},
			{Type: "interface", ID: "API-001", Title: "Interface"},
		} {
			if _, err := engine.CreateEntity(ctx, entity); err != nil {
				t.Fatalf("create %s: %v", entity.ID, err)
			}
		}
		for _, relation := range []specgraph.AddRelationRequest{
			{From: "API-001", To: "REQ-001", Type: "implements"},
			{From: "API-001", To: "REQ-001", Type: "references"},
		} {
			if _, err := engine.AddRelation(ctx, relation); err != nil {
				t.Fatalf("add %s: %v", relation.Type, err)
			}
		}

		result := decodeRPCResult[deleteRPCResult](t, callRPC(t, NewDispatcher(engine), "relation.delete", map[string]any{
			"from": "API-001", "to": "REQ-001", "type": "implements",
		}))
		if result.Deleted != "API-001->REQ-001[implements]" {
			t.Errorf("deleted = %q; want API-001->REQ-001[implements]", result.Deleted)
		}
		persisted, count, err := engine.ListRelations(ctx, specgraph.ListRelationsRequest{From: "API-001", To: "REQ-001"})
		if err != nil {
			t.Fatalf("list persisted relations: %v", err)
		}
		if count != 1 || len(persisted) != 1 || persisted[0].Type != "references" {
			t.Errorf("persisted relations = %+v, count = %d; want only references", persisted, count)
		}
	})
}
