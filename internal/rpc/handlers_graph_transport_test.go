package rpc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/graph"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/validate"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func seedGraphTransportFixture(t *testing.T, engine *specgraph.Engine) {
	t.Helper()
	ctx := context.Background()
	for _, entity := range []specgraph.CreateEntityRequest{
		{Type: "plan", ID: "PLN-001", Title: "Transport plan", Status: "active"},
		{Type: "phase", ID: "PHS-001", Title: "Transport phase", Metadata: json.RawMessage(`{"goal":"Exercise transport","order":1}`)},
		{Type: "requirement", ID: "REQ-001", Title: "Transport requirement"},
		{Type: "interface", ID: "API-001", Title: "Transport interface"},
		{Type: "test", ID: "TST-001", Title: "Transport test"},
		{Type: "question", ID: "QST-001", Title: "Open question"},
		{Type: "assumption", ID: "ASM-001", Title: "Open assumption"},
		{Type: "risk", ID: "RSK-001", Title: "Open risk"},
	} {
		if _, err := engine.CreateEntity(ctx, entity); err != nil {
			t.Fatalf("create %s: %v", entity.ID, err)
		}
	}
	for _, relation := range []specgraph.AddRelationRequest{
		{From: "PHS-001", To: "PLN-001", Type: "belongs_to"},
		{From: "PHS-001", To: "REQ-001", Type: "covers"},
		{From: "PHS-001", To: "API-001", Type: "delivers"},
		{From: "API-001", To: "REQ-001", Type: "implements"},
		{From: "TST-001", To: "API-001", Type: "verifies"},
	} {
		if _, err := engine.AddRelation(ctx, relation); err != nil {
			t.Fatalf("add %s %s -> %s: %v", relation.Type, relation.From, relation.To, err)
		}
	}
}

func seedValidationTransportFixture(t *testing.T, engine *specgraph.Engine) {
	t.Helper()
	ctx := context.Background()
	for _, entity := range []specgraph.CreateEntityRequest{
		{Type: "question", ID: "QST-001", Title: "Open question"},
		{Type: "assumption", ID: "ASM-001", Title: "Open assumption"},
		{Type: "phase", ID: "PHS-001", Title: "Orphan phase"},
	} {
		if _, err := engine.CreateEntity(ctx, entity); err != nil {
			t.Fatalf("create %s: %v", entity.ID, err)
		}
	}
}

func TestGraphQueryTransportContracts(t *testing.T) {
	engine := newTestEngine(t)
	seedGraphTransportFixture(t, engine)
	dispatcher := NewDispatcher(engine)

	t.Run("scope returns entities from the requested layer", func(t *testing.T) {
		arch := decodeRPCResult[graph.QueryScopeResult](t, callRPC(t, dispatcher, "query.scope", map[string]any{
			"phase_id": "PHS-001", "layer": "arch",
		}))
		if arch.PhaseID != "PHS-001" {
			t.Errorf("phase ID = %q; want PHS-001", arch.PhaseID)
		}
		archIDs := make(map[string]bool, len(arch.Entities))
		for _, entity := range arch.Entities {
			archIDs[entity.ID] = true
		}
		if len(archIDs) != 2 || !archIDs["REQ-001"] || !archIDs["API-001"] {
			t.Errorf("arch scope entities = %v; want REQ-001 and API-001", archIDs)
		}

		exec := decodeRPCResult[graph.QueryScopeResult](t, callRPC(t, dispatcher, "query.scope", map[string]any{
			"phase_id": "PHS-001", "layer": "exec",
		}))
		if len(exec.Entities) != 0 {
			t.Errorf("exec scope entities = %+v; want none", exec.Entities)
		}
	})

	t.Run("neighbors return entities within the requested depth", func(t *testing.T) {
		depthOne := decodeRPCResult[graph.NeighborResult](t, callRPC(t, dispatcher, "query.neighbors", map[string]any{
			"entity_id": "TST-001", "depth": 1,
		}))
		depthOneIDs := make(map[string]int, len(depthOne.Entities))
		for _, entity := range depthOne.Entities {
			depthOneIDs[entity.Entity.ID] = entity.Depth
		}
		if depthOne.Center != "TST-001" || len(depthOneIDs) != 2 || depthOneIDs["API-001"] != 1 {
			t.Errorf("depth-one neighbors = center %q, entities %v; want TST-001 and API-001 at depth 1", depthOne.Center, depthOneIDs)
		}
		if _, found := depthOneIDs["REQ-001"]; found {
			t.Error("REQ-001 appeared at depth 1; want it outside the requested depth")
		}

		depthTwo := decodeRPCResult[graph.NeighborResult](t, callRPC(t, dispatcher, "query.neighbors", map[string]any{
			"entity_id": "TST-001", "depth": 2,
		}))
		depthTwoIDs := make(map[string]int, len(depthTwo.Entities))
		for _, entity := range depthTwo.Entities {
			depthTwoIDs[entity.Entity.ID] = entity.Depth
		}
		if depth, found := depthTwoIDs["REQ-001"]; !found || depth != 2 {
			t.Errorf("depth-two neighbors = %v; want REQ-001 at depth 2", depthTwoIDs)
		}
	})

	t.Run("path results honor the requested layer", func(t *testing.T) {
		arch := decodeRPCResult[graph.QueryPathResult](t, callRPC(t, dispatcher, "query.path", map[string]any{
			"from_id": "TST-001", "to_id": "REQ-001", "layer": "arch",
		}))
		if !arch.Found || len(arch.Path) != 3 {
			t.Fatalf("arch path = %+v; want a three-node path", arch)
		}
		for index, wantID := range []string{"TST-001", "API-001", "REQ-001"} {
			if arch.Path[index].EntityID != wantID {
				t.Errorf("arch path[%d] = %q; want %q", index, arch.Path[index].EntityID, wantID)
			}
		}

		mapping := decodeRPCResult[graph.QueryPathResult](t, callRPC(t, dispatcher, "query.path", map[string]any{
			"from_id": "TST-001", "to_id": "REQ-001", "layer": "mapping",
		}))
		if mapping.Found || len(mapping.Path) != 0 {
			t.Errorf("mapping path = %+v; want no path", mapping)
		}
	})

	t.Run("unresolved results honor the requested type", func(t *testing.T) {
		all := decodeRPCResult[graph.QueryUnresolvedResult](t, callRPC(t, dispatcher, "query.unresolved", map[string]any{}))
		if all.Count != 3 {
			t.Errorf("all unresolved count = %d; want 3", all.Count)
		}

		risk := decodeRPCResult[graph.QueryUnresolvedResult](t, callRPC(t, dispatcher, "query.unresolved", map[string]any{
			"type": "risk",
		}))
		if risk.Count != 1 || len(risk.Entities) != 1 || risk.Entities[0].ID != "RSK-001" || risk.Entities[0].Type != "risk" {
			t.Errorf("risk unresolved result = %+v; want only RSK-001", risk)
		}
	})
}

func TestGraphImpactTransportContracts(t *testing.T) {
	engine := newTestEngine(t)
	seedGraphTransportFixture(t, engine)
	dispatcher := NewDispatcher(engine)

	t.Run("impact follows the requested relation types", func(t *testing.T) {
		result := decodeRPCResult[graph.ImpactResult](t, callRPC(t, dispatcher, "impact", map[string]any{
			"sources": []string{"REQ-001"}, "follow": []string{"implements"},
		}))
		if len(result.Sources) != 1 || result.Sources[0] != "REQ-001" {
			t.Errorf("sources = %v; want [REQ-001]", result.Sources)
		}
		if result.Summary.Total != 1 || len(result.Affected) != 1 {
			t.Fatalf("impact result = %+v; want one affected entity", result)
		}
		affected := result.Affected[0]
		if affected.ID != "API-001" || len(affected.RelationChain) != 1 || affected.RelationChain[0] != "implements" {
			t.Errorf("affected entity = %+v; want API-001 via implements", affected)
		}
	})

	t.Run("impact results honor the requested layer", func(t *testing.T) {
		arch := decodeRPCResult[graph.ImpactResult](t, callRPC(t, dispatcher, "impact", map[string]any{
			"sources": []string{"REQ-001"}, "layer": "arch",
		}))
		archIDs := make(map[string]bool, len(arch.Affected))
		for _, affected := range arch.Affected {
			archIDs[affected.ID] = true
		}
		if len(archIDs) != 2 || !archIDs["API-001"] || !archIDs["TST-001"] || archIDs["PHS-001"] {
			t.Errorf("arch impact entities = %v; want API-001 and TST-001", archIDs)
		}

		mapping := decodeRPCResult[graph.ImpactResult](t, callRPC(t, dispatcher, "impact", map[string]any{
			"sources": []string{"REQ-001"}, "layer": "mapping",
		}))
		mappingIDs := make(map[string]bool, len(mapping.Affected))
		for _, affected := range mapping.Affected {
			mappingIDs[affected.ID] = true
		}
		if len(mappingIDs) != 2 || !mappingIDs["PHS-001"] || !mappingIDs["API-001"] || mappingIDs["TST-001"] {
			t.Errorf("mapping impact entities = %v; want PHS-001 and API-001", mappingIDs)
		}
	})

	t.Run("impact applies requested dimension and severity", func(t *testing.T) {
		result := decodeRPCResult[graph.ImpactResult](t, callRPC(t, dispatcher, "impact", map[string]any{
			"sources": []string{"REQ-001"}, "follow": []string{"implements"},
			"dimension": "planning", "min_severity": "high",
		}))
		if result.Summary.Total != 0 || len(result.Affected) != 0 {
			t.Errorf("planning high-severity impact = %+v; want no affected entities", result)
		}
	})
}

func TestGraphValidateTransportContracts(t *testing.T) {
	t.Run("phase satisfaction includes requested reference advisories", func(t *testing.T) {
		engine := newTestEngine(t)
		ctx := context.Background()
		for _, entity := range []specgraph.CreateEntityRequest{
			{Type: "phase", ID: "PHS-001", Title: "Validation phase"},
			{Type: "requirement", ID: "REQ-001", Title: "Covered requirement"},
			{Type: "decision", ID: "DEC-001", Title: "Referenced decision"},
		} {
			if _, err := engine.CreateEntity(ctx, entity); err != nil {
				t.Fatalf("create %s: %v", entity.ID, err)
			}
		}
		for _, relation := range []specgraph.AddRelationRequest{
			{From: "PHS-001", To: "REQ-001", Type: "covers"},
			{From: "REQ-001", To: "DEC-001", Type: "references"},
		} {
			if _, err := engine.AddRelation(ctx, relation); err != nil {
				t.Fatalf("add %s: %v", relation.Type, err)
			}
		}

		dispatcher := NewDispatcher(engine)
		withoutReferences := decodeRPCResult[validate.ValidateResult](t, callRPC(t, dispatcher, "validate", map[string]any{
			"checks": []string{"phase_satisfaction"}, "phase": "PHS-001",
			"include_references": false,
		}))
		if len(withoutReferences.Satisfaction) != 1 || withoutReferences.Satisfaction[0].AdvisoryCount != 0 {
			t.Fatalf("validation without references = %+v; want one report without advisory members", withoutReferences.Satisfaction)
		}

		withReferences := decodeRPCResult[validate.ValidateResult](t, callRPC(t, dispatcher, "validate", map[string]any{
			"checks": []string{"phase_satisfaction"}, "phase": "PHS-001",
			"include_references": true,
		}))
		if len(withReferences.Satisfaction) != 1 || withReferences.Satisfaction[0].AdvisoryCount != 1 {
			t.Fatalf("validation with references = %+v; want one advisory member", withReferences.Satisfaction)
		}
		foundAdvisory := false
		for _, item := range withReferences.Satisfaction[0].Items {
			if item.EntityID == "DEC-001" && item.Status == validate.SatisfactionAdvisory {
				foundAdvisory = true
			}
		}
		if !foundAdvisory {
			t.Errorf("satisfaction items = %+v; want DEC-001 advisory item", withReferences.Satisfaction[0].Items)
		}
		if len(withReferences.Issues) == 0 {
			t.Fatal("phase_satisfaction produced no issues for an undelivered requirement")
		}
		for _, issue := range withReferences.Issues {
			if issue.Check != "phase_satisfaction" {
				t.Errorf("issue check = %q; want phase_satisfaction", issue.Check)
			}
		}
	})

	t.Run("validation returns only issues for the requested entity", func(t *testing.T) {
		engine := newTestEngine(t)
		seedValidationTransportFixture(t, engine)
		dispatcher := NewDispatcher(engine)

		all := decodeRPCResult[validate.ValidateResult](t, callRPC(t, dispatcher, "validate", map[string]any{
			"checks": []string{"unresolved"}, "layer": "arch",
		}))
		issueEntities := make(map[string]bool, len(all.Issues))
		for _, issue := range all.Issues {
			issueEntities[issue.Entity] = true
		}
		if len(issueEntities) != 2 || !issueEntities["QST-001"] || !issueEntities["ASM-001"] {
			t.Fatalf("unresolved issue entities = %v; want QST-001 and ASM-001", issueEntities)
		}

		scoped := decodeRPCResult[validate.ValidateResult](t, callRPC(t, dispatcher, "validate", map[string]any{
			"checks": []string{"unresolved"}, "layer": "arch", "entity_id": "QST-001",
		}))
		if len(scoped.Issues) != 1 || scoped.Issues[0].Entity != "QST-001" || scoped.Issues[0].Check != "unresolved" {
			t.Errorf("scoped validation issues = %+v; want only QST-001 unresolved", scoped.Issues)
		}
	})

	t.Run("validation returns issues from the requested layer", func(t *testing.T) {
		engine := newTestEngine(t)
		seedValidationTransportFixture(t, engine)
		dispatcher := NewDispatcher(engine)

		arch := decodeRPCResult[validate.ValidateResult](t, callRPC(t, dispatcher, "validate", map[string]any{
			"layer": "arch",
		}))
		if len(arch.Issues) == 0 {
			t.Fatal("arch validation returned no issues")
		}
		for _, issue := range arch.Issues {
			if issue.Layer != "arch" {
				t.Errorf("arch validation issue layer = %q; want arch", issue.Layer)
			}
		}

		exec := decodeRPCResult[validate.ValidateResult](t, callRPC(t, dispatcher, "validate", map[string]any{
			"layer": "exec",
		}))
		if len(exec.Issues) != 1 || exec.Issues[0].Entity != "PHS-001" || exec.Issues[0].Check != "orphan_phases" || exec.Issues[0].Layer != "exec" {
			t.Errorf("exec validation issues = %+v; want PHS-001 orphan_phases", exec.Issues)
		}
	})
}

func TestGraphExportTransportContracts(t *testing.T) {
	engine := newTestEngine(t)
	seedGraphTransportFixture(t, engine)
	dispatcher := NewDispatcher(engine)

	t.Run("JSON export limits a centered graph to the requested depth", func(t *testing.T) {
		result := decodeRPCResult[exportResult](t, callRPC(t, dispatcher, "export", map[string]any{
			"format": "json", "center": "TST-001", "depth": 1,
		}))
		if result.Format != "json" {
			t.Errorf("format = %q; want json", result.Format)
		}
		var exported jsoncontract.ExportJSONResult
		if err := json.Unmarshal([]byte(result.Data), &exported); err != nil {
			t.Fatalf("unmarshal exported JSON: %v", err)
		}
		entityIDs := make(map[string]bool, len(exported.Entities))
		for _, entity := range exported.Entities {
			entityIDs[entity.ID] = true
		}
		if len(entityIDs) != 2 || !entityIDs["TST-001"] || !entityIDs["API-001"] || entityIDs["REQ-001"] || entityIDs["PHS-001"] {
			t.Errorf("exported entity IDs = %v; want only TST-001 and API-001", entityIDs)
		}
	})

	t.Run("JSON export includes only the requested layer", func(t *testing.T) {
		archResult := decodeRPCResult[exportResult](t, callRPC(t, dispatcher, "export", map[string]any{
			"format": "json", "layer": "arch",
		}))
		var arch jsoncontract.ExportJSONResult
		if err := json.Unmarshal([]byte(archResult.Data), &arch); err != nil {
			t.Fatalf("unmarshal arch export: %v", err)
		}
		archEntityIDs := make(map[string]bool, len(arch.Entities))
		for _, entity := range arch.Entities {
			archEntityIDs[entity.ID] = true
		}
		archRelationTypes := make(map[string]bool, len(arch.Relations))
		for _, relation := range arch.Relations {
			archRelationTypes[relation.Type] = true
		}
		if !archEntityIDs["REQ-001"] || !archEntityIDs["API-001"] || archEntityIDs["PHS-001"] || archEntityIDs["PLN-001"] {
			t.Errorf("arch export entities = %v; want arch entities without PHS-001 or PLN-001", archEntityIDs)
		}
		if len(arch.Relations) != 2 || !archRelationTypes["implements"] || !archRelationTypes["verifies"] {
			t.Errorf("arch export relations = %+v; want implements and verifies", arch.Relations)
		}

		execResult := decodeRPCResult[exportResult](t, callRPC(t, dispatcher, "export", map[string]any{
			"format": "json", "layer": "exec",
		}))
		var exec jsoncontract.ExportJSONResult
		if err := json.Unmarshal([]byte(execResult.Data), &exec); err != nil {
			t.Fatalf("unmarshal exec export: %v", err)
		}
		execEntityIDs := make(map[string]bool, len(exec.Entities))
		for _, entity := range exec.Entities {
			execEntityIDs[entity.ID] = true
		}
		if len(execEntityIDs) != 2 || !execEntityIDs["PHS-001"] || !execEntityIDs["PLN-001"] || execEntityIDs["REQ-001"] || execEntityIDs["API-001"] {
			t.Errorf("exec export entities = %v; want only PHS-001 and PLN-001", execEntityIDs)
		}
		if len(exec.Relations) != 1 || exec.Relations[0].Type != "belongs_to" {
			t.Errorf("exec export relations = %+v; want belongs_to", exec.Relations)
		}
	})

	t.Run("export returns the selected DOT format", func(t *testing.T) {
		result := decodeRPCResult[exportResult](t, callRPC(t, dispatcher, "export", map[string]any{
			"format": "dot", "center": "TST-001", "depth": 1,
		}))
		if result.Format != "dot" {
			t.Errorf("format = %q; want dot", result.Format)
		}
		if !strings.Contains(result.Data, "digraph spec_graph {") || !strings.Contains(result.Data, "TST-001") || !strings.Contains(result.Data, "API-001") {
			t.Errorf("DOT export omitted expected graph content: %q", result.Data)
		}
	})
}

func TestGraphWriteTransportContracts(t *testing.T) {
	t.Run("phase next activates the selected draft phase", func(t *testing.T) {
		engine := newTestEngine(t)
		seedGraphTransportFixture(t, engine)

		result := decodeRPCResult[specgraph.PhaseNextResult](t, callRPC(t, NewDispatcher(engine), "phase.next", map[string]any{
			"activate": true,
		}))
		if !result.Activated || result.Phase.ID != "PHS-001" || result.Phase.Status != "active" {
			t.Errorf("phase.next result = %+v; want activated PHS-001", result)
		}
		if result.Phase.Goal != "Exercise transport" || result.Phase.Order != 1 {
			t.Errorf("phase details = %+v; want goal Exercise transport and order 1", result.Phase)
		}
		persisted, err := engine.GetEntity(context.Background(), "PHS-001")
		if err != nil {
			t.Fatalf("get activated phase: %v", err)
		}
		if persisted.Status != "active" {
			t.Errorf("persisted phase status = %q; want active", persisted.Status)
		}
	})

	t.Run("bootstrap import preserves nested candidate decisions", func(t *testing.T) {
		engine := newTestEngine(t)
		ctx := context.Background()
		if _, err := engine.CreateEntity(ctx, specgraph.CreateEntityRequest{
			Type: "requirement", ID: "REQ-001", Title: "Existing requirement",
		}); err != nil {
			t.Fatalf("create existing requirement: %v", err)
		}

		result := decodeRPCResult[specgraph.BootstrapImportResult](t, callRPC(t, NewDispatcher(engine), "bootstrap.import", map[string]any{
			"entities": []map[string]any{
				{"id": "REQ-001", "type": "requirement", "title": "Replacement title", "confidence": 0.9},
				{"id": "API-001", "type": "interface", "title": "Imported interface", "confidence": 0.9},
				{"id": "REQ-002", "type": "requirement", "title": "Low confidence requirement", "confidence": 0.4},
			},
			"relations": []map[string]any{
				{"from": "API-001", "to": "REQ-001", "type": "implements", "confidence": 0.9},
				{"from": "REQ-001", "to": "API-001", "type": "implements", "confidence": 0.9},
			},
		}))
		created := make(map[string]bool, len(result.Created))
		for _, id := range result.Created {
			created[id] = true
		}
		if !created["API-001"] || !created["API-001:REQ-001:implements"] {
			t.Errorf("created = %v; want imported interface and valid relation", result.Created)
		}
		skipped := make(map[string]string, len(result.Skipped))
		for _, item := range result.Skipped {
			skipped[item.ID] = item.Reason
		}
		for id, wantReason := range map[string]string{
			"REQ-001":                    "already exists",
			"REQ-002":                    "low confidence",
			"REQ-001:API-001:implements": "invalid edge",
		} {
			if skipped[id] != wantReason {
				t.Errorf("skip reason for %q = %q; want %q", id, skipped[id], wantReason)
			}
		}

		existing, err := engine.GetEntity(ctx, "REQ-001")
		if err != nil {
			t.Fatalf("get existing requirement: %v", err)
		}
		if existing.Title != "Existing requirement" {
			t.Errorf("existing title = %q; want Existing requirement", existing.Title)
		}
		imported, err := engine.GetEntity(ctx, "API-001")
		if err != nil {
			t.Fatalf("get imported interface: %v", err)
		}
		if imported.Title != "Imported interface" {
			t.Errorf("imported title = %q; want Imported interface", imported.Title)
		}
		if _, err := engine.GetEntity(ctx, "REQ-002"); !specgraph.IsNotFound(err) {
			t.Errorf("low-confidence candidate lookup error = %v; want not found", err)
		}
		relations, count, err := engine.ListRelations(ctx, specgraph.ListRelationsRequest{
			From: "API-001", To: "REQ-001", Type: "implements",
		})
		if err != nil {
			t.Fatalf("list imported relation: %v", err)
		}
		if count != 1 || len(relations) != 1 {
			t.Errorf("imported relations = %+v, count = %d; want one", relations, count)
		}
	})
}
