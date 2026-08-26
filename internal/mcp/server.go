// Package mcp exposes spec-graph over the Model Context Protocol. Tools are
// shaped around the units an agent works in.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/tyeongkim/spec-graph/internal/gate"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

// NewSpecGraphServer builds the MCP server exposing spec-graph to an agent.
func NewSpecGraphServer(engine *specgraph.Engine, version string) *server.MCPServer {
	s := server.NewMCPServer("spec-graph", version, server.WithToolCapabilities(true))

	addTool(s, planStatusTool(), handlePlanStatus(engine))
	addTool(s, phaseBriefTool(), handlePhaseBrief(engine))
	addTool(s, phaseGateTool(), handlePhaseGate(engine))
	addTool(s, changeImpactTool(), handleChangeImpact(engine))
	addTool(s, getEntityTool(), handleGetEntity(engine))
	addTool(s, listEntitiesTool(), handleListEntities(engine))
	addTool(s, listRelationsTool(), handleListRelations(engine))
	addTool(s, queryPathTool(), handleQueryPath(engine))
	addTool(s, applyBatchTool(), handleApplyBatch(engine))
	addTool(s, updateEntityTool(), handleUpdateEntity(engine))
	addTool(s, deleteEntityTool(), handleDeleteEntity(engine))
	addTool(s, deleteRelationTool(), handleDeleteRelation(engine))
	addTool(s, nextPhaseTool(), handleNextPhase(engine))

	return s
}

// addTool binds a handler that receives arguments already decoded into Args.
func addTool[Args any](s *server.MCPServer, tool mcp.Tool, handle func(context.Context, Args) (any, error)) {
	s.AddTool(tool, mcp.NewTypedToolHandler(
		func(ctx context.Context, _ mcp.CallToolRequest, args Args) (*mcp.CallToolResult, error) {
			result, err := handle(ctx, args)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, err := json.Marshal(result)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("marshal result: %s", err.Error())), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	))
}

// readOnly marks a tool as making no change. Both hints are set because mcp-go
// defaults destructiveHint to true, which would otherwise contradict readOnlyHint
// on a query tool.
func readOnly() mcp.ToolOption {
	return func(tool *mcp.Tool) {
		mcp.WithReadOnlyHintAnnotation(true)(tool)
		mcp.WithDestructiveHintAnnotation(false)(tool)
	}
}

type phaseArgs struct {
	PhaseID string `json:"phase_id"`
}

type noArgs struct{}

func planStatusTool() mcp.Tool {
	return mcp.NewTool("plan_status",
		mcp.WithDescription("Report the active plan and the phases open within it. Call this first to find what to work on."),
		readOnly(),
	)
}

func handlePlanStatus(engine *specgraph.Engine) func(context.Context, noArgs) (any, error) {
	return func(ctx context.Context, _ noArgs) (any, error) {
		return planStatus(ctx, engine)
	}
}

func phaseBriefTool() mcp.Tool {
	return mcp.NewTool("phase_brief",
		mcp.WithDescription("Return everything needed to start work on a phase: plan, ordered task contracts, prerequisites, effective scope and delivery, ready and blocked task IDs, plus the phase-scoped mapping and task-graph issues."),
		mcp.WithString("phase_id",
			mcp.Required(),
			mcp.Description("Phase entity ID, e.g. PHS-001"),
		),
		readOnly(),
	)
}

func handlePhaseBrief(engine *specgraph.Engine) func(context.Context, phaseArgs) (any, error) {
	return func(ctx context.Context, args phaseArgs) (any, error) {
		return phaseBrief(ctx, engine, args.PhaseID)
	}
}

func phaseGateTool() mcp.Tool {
	return mcp.NewTool("phase_gate",
		mcp.WithDescription("Run every graph-level check governing phase resolution and report what blocks it. This does not change state: resolving a phase also requires passing tests, a working build, and per-entity code verification, then an explicit update_entity call setting status to resolved."),
		mcp.WithString("phase_id",
			mcp.Required(),
			mcp.Description("Phase entity ID to check for resolution readiness"),
		),
		readOnly(),
	)
}

func handlePhaseGate(engine *specgraph.Engine) func(context.Context, phaseArgs) (any, error) {
	return func(ctx context.Context, args phaseArgs) (any, error) {
		return phaseGate(ctx, engine, args.PhaseID)
	}
}

type changeImpactArgs struct {
	Sources     []string `json:"sources"`
	Follow      []string `json:"follow"`
	Dimension   string   `json:"dimension"`
	MinSeverity string   `json:"min_severity"`
	Layer       string   `json:"layer"`
}

func changeImpactTool() mcp.Tool {
	return mcp.NewTool("change_impact",
		mcp.WithDescription("Analyze what a change to the given entities affects, with the direct neighbors of each source. Call this before modifying an entity."),
		mcp.WithArray("sources",
			mcp.Required(),
			mcp.Description("Entity IDs the change starts from"),
			mcp.WithStringItems(),
		),
		mcp.WithArray("follow",
			mcp.Description("Restrict traversal to these relation types. Omit to follow every type."),
			mcp.WithStringEnumItems(relationTypeNames()),
		),
		mcp.WithString("dimension",
			mcp.Description("Restrict scoring to one dimension. Omit for all."),
			mcp.Enum("structural", "behavioral", "planning"),
		),
		mcp.WithString("min_severity",
			mcp.Description("Drop affected entities below this severity"),
			mcp.Enum("high", "medium", "low"),
		),
		layerProperty(),
		readOnly(),
	)
}

func handleChangeImpact(engine *specgraph.Engine) func(context.Context, changeImpactArgs) (any, error) {
	return func(ctx context.Context, args changeImpactArgs) (any, error) {
		return changeImpact(ctx, engine, specgraph.ImpactRequest{
			Sources:     args.Sources,
			Follow:      args.Follow,
			Dimension:   args.Dimension,
			MinSeverity: args.MinSeverity,
			Layer:       args.Layer,
		})
	}
}

type entityIDArgs struct {
	ID string `json:"id"`
}

func getEntityTool() mcp.Tool {
	return mcp.NewTool("get_entity",
		mcp.WithDescription("Fetch one entity by ID"),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Entity ID"),
		),
		readOnly(),
	)
}

func handleGetEntity(engine *specgraph.Engine) func(context.Context, entityIDArgs) (any, error) {
	return func(ctx context.Context, args entityIDArgs) (any, error) {
		entity, err := engine.GetEntity(ctx, args.ID)
		if err != nil {
			return nil, err
		}
		return jsoncontract.EntityResponse{Entity: entity}, nil
	}
}

type listEntitiesArgs struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Layer  string `json:"layer"`
}

func listEntitiesTool() mcp.Tool {
	return mcp.NewTool("list_entities",
		mcp.WithDescription("List entities, optionally filtered by type, status, or layer. Query before creating to avoid duplicates."),
		mcp.WithString("type",
			mcp.Description("Filter by entity type"),
			mcp.Enum(entityTypeNames()...),
		),
		mcp.WithString("status",
			mcp.Description("Filter by status"),
			mcp.Enum(statusNames()...),
		),
		layerProperty(),
		readOnly(),
	)
}

func handleListEntities(engine *specgraph.Engine) func(context.Context, listEntitiesArgs) (any, error) {
	return func(ctx context.Context, args listEntitiesArgs) (any, error) {
		entities, count, err := engine.ListEntities(ctx, specgraph.ListEntitiesRequest{
			Type:   args.Type,
			Status: args.Status,
			Layer:  args.Layer,
		})
		if err != nil {
			return nil, err
		}
		return jsoncontract.EntityListResponse{Entities: entities, Count: count}, nil
	}
}

type listRelationsArgs struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Type  string `json:"type"`
	Layer string `json:"layer"`
}

func listRelationsTool() mcp.Tool {
	return mcp.NewTool("list_relations",
		mcp.WithDescription("List relations, optionally filtered by endpoint, type, or layer"),
		mcp.WithString("from", mcp.Description("Filter by source entity ID")),
		mcp.WithString("to", mcp.Description("Filter by target entity ID")),
		mcp.WithString("type",
			mcp.Description("Filter by relation type"),
			mcp.Enum(relationTypeNames()...),
		),
		layerProperty(),
		readOnly(),
	)
}

func handleListRelations(engine *specgraph.Engine) func(context.Context, listRelationsArgs) (any, error) {
	return func(ctx context.Context, args listRelationsArgs) (any, error) {
		relations, count, err := engine.ListRelations(ctx, specgraph.ListRelationsRequest{
			From:  args.From,
			To:    args.To,
			Type:  args.Type,
			Layer: args.Layer,
		})
		if err != nil {
			return nil, err
		}
		return jsoncontract.RelationListResponse{Relations: relations, Count: count}, nil
	}
}

type queryPathArgs struct {
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
	Layer  string `json:"layer"`
}

func queryPathTool() mcp.Tool {
	return mcp.NewTool("query_path",
		mcp.WithDescription("Find the shortest relation path between two entities"),
		mcp.WithString("from_id",
			mcp.Required(),
			mcp.Description("Source entity ID"),
		),
		mcp.WithString("to_id",
			mcp.Required(),
			mcp.Description("Target entity ID"),
		),
		layerProperty(),
		readOnly(),
	)
}

func handleQueryPath(engine *specgraph.Engine) func(context.Context, queryPathArgs) (any, error) {
	return func(ctx context.Context, args queryPathArgs) (any, error) {
		return engine.QueryPath(ctx, specgraph.QueryPathRequest{
			FromID: args.FromID,
			ToID:   args.ToID,
			Layer:  args.Layer,
		})
	}
}

type batchEntityArgs struct {
	Ref         string          `json:"ref"`
	Type        string          `json:"type"`
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Status      string          `json:"status"`
	Metadata    json.RawMessage `json:"metadata"`
}

type batchRelationArgs struct {
	From     string          `json:"from"`
	To       string          `json:"to"`
	Type     string          `json:"type"`
	Weight   float64         `json:"weight"`
	Metadata json.RawMessage `json:"metadata"`
}

type applyBatchArgs struct {
	Entities  []batchEntityArgs   `json:"entities"`
	Relations []batchRelationArgs `json:"relations"`
}

func applyBatchTool() mcp.Tool {
	return mcp.NewTool("apply_batch",
		mcp.WithDescription("Create entities and relations as one unit. Any failure aborts the whole batch and leaves the graph untouched, so this is how a plan, or a set of artifacts discovered during implementation, is registered. Every entity is created before any relation."),
		mcp.WithArray("entities",
			mcp.Description("Entities to create"),
			mcp.Items(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"type": map[string]any{
						"type":        "string",
						"description": "Entity type",
						"enum":        entityTypeNames(),
					},
					"id": map[string]any{
						"type":        "string",
						"description": "Entity ID. Omit to have one generated; its prefix must match type when supplied.",
					},
					"ref": map[string]any{
						"type":        "string",
						"description": "Name for this entity within the batch, so a relation below can point at a generated ID. Must not look like an entity ID. Unnecessary when id is supplied.",
					},
					"title":       map[string]any{"type": "string", "description": "Entity title"},
					"description": map[string]any{"type": "string", "description": "Longer description"},
					"status": map[string]any{
						"type":        "string",
						"description": "Initial status. Defaults to draft.",
						"enum":        statusNames(),
					},
					"metadata": map[string]any{"type": "object", "description": "Type-specific metadata"},
				},
				"required": []string{"type", "title"},
			}),
		),
		mcp.WithArray("relations",
			mcp.Description("Relations to add once the entities exist"),
			mcp.Items(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"from": map[string]any{
						"type":        "string",
						"description": "Source: an entity ID, or a ref declared in this batch",
					},
					"to": map[string]any{
						"type":        "string",
						"description": "Target: an entity ID, or a ref declared in this batch",
					},
					"type": map[string]any{
						"type":        "string",
						"description": "Relation type",
						"enum":        relationTypeNames(),
					},
					"weight":   map[string]any{"type": "number", "description": "Relation weight. Defaults to 1."},
					"metadata": map[string]any{"type": "object", "description": "Relation metadata"},
				},
				"required": []string{"from", "to", "type"},
			}),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
	)
}

// batchEntityResult reports the generated ID next to the ref that named it.
// specgraph.BatchResult carries no JSON tags, so returning it directly would put
// Go field names on the wire while every other tool emits snake_case.
type batchEntityResult struct {
	Ref    string       `json:"ref,omitempty"`
	Entity model.Entity `json:"entity"`
}

type batchResult struct {
	Entities  []batchEntityResult `json:"entities"`
	Relations []model.Relation    `json:"relations"`
}

func handleApplyBatch(engine *specgraph.Engine) func(context.Context, applyBatchArgs) (any, error) {
	return func(ctx context.Context, args applyBatchArgs) (any, error) {
		request := specgraph.BatchRequest{
			Entities:  make([]specgraph.BatchEntity, 0, len(args.Entities)),
			Relations: make([]specgraph.BatchRelation, 0, len(args.Relations)),
		}
		for _, entity := range args.Entities {
			request.Entities = append(request.Entities, specgraph.BatchEntity{
				Ref: entity.Ref,
				CreateEntityRequest: specgraph.CreateEntityRequest{
					Type:        entity.Type,
					ID:          entity.ID,
					Title:       entity.Title,
					Description: entity.Description,
					Status:      entity.Status,
					Metadata:    entity.Metadata,
				},
			})
		}
		for _, relation := range args.Relations {
			request.Relations = append(request.Relations, specgraph.BatchRelation{
				From:     relation.From,
				To:       relation.To,
				Type:     relation.Type,
				Weight:   relation.Weight,
				Metadata: relation.Metadata,
			})
		}

		applied, err := engine.ApplyBatch(ctx, request)
		if err != nil {
			return nil, err
		}

		created := make([]batchEntityResult, len(applied.Entities))
		for i, item := range applied.Entities {
			created[i] = batchEntityResult{Ref: item.Ref, Entity: item.Entity}
		}
		return batchResult{Entities: created, Relations: applied.Relations}, nil
	}
}

type updateEntityArgs struct {
	ID          string           `json:"id"`
	Title       *string          `json:"title"`
	Description *string          `json:"description"`
	Status      *string          `json:"status"`
	Metadata    *json.RawMessage `json:"metadata"`
	Force       bool             `json:"force"`
	Reason      string           `json:"reason"`
}

// updateEntityResult states the outcome next to the entity, so a blocked
// transition is not mistaken for a successful write. It mirrors the RPC shape
// rather than jsoncontract.EntityUpdateSuccessResponse, which reports gate
// findings as issue lists instead of the report itself.
type updateEntityResult struct {
	Entity  model.Entity `json:"entity"`
	Outcome string       `json:"outcome"`
	Blocked bool         `json:"blocked"`
	Gate    *gate.Report `json:"gate_report,omitempty"`
}

func updateEntityTool() mcp.Tool {
	return mcp.NewTool("update_entity",
		mcp.WithDescription("Change an entity's title, description, status, or metadata. This is also how an entity is deprecated and how a phase or task is resolved; a status change runs the applicable gate, which can block the write."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Entity ID to update"),
		),
		mcp.WithString("title", mcp.Description("Replacement title")),
		mcp.WithString("description", mcp.Description("Replacement description")),
		mcp.WithString("status",
			mcp.Description("Replacement status. Subject to gate checks."),
			mcp.Enum(statusNames()...),
		),
		mcp.WithObject("metadata", mcp.Description("Replacement metadata, replacing the existing object entirely")),
		mcp.WithBoolean("force",
			mcp.Description("Accept completion findings a gate reports. Structural blocks are never bypassed. Requires reason."),
		),
		mcp.WithString("reason",
			mcp.Description("Why the change is being made. Required when deprecating a task, and when force accepts gate findings."),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
	)
}

func handleUpdateEntity(engine *specgraph.Engine) func(context.Context, updateEntityArgs) (any, error) {
	return func(ctx context.Context, args updateEntityArgs) (any, error) {
		result, err := engine.UpdateEntity(ctx, specgraph.UpdateEntityRequest{
			ID:          args.ID,
			Title:       args.Title,
			Description: args.Description,
			Status:      args.Status,
			Metadata:    args.Metadata,
			Force:       args.Force,
			Reason:      args.Reason,
		})
		if err != nil {
			return nil, err
		}
		return updateEntityResult{
			Entity:  result.Entity,
			Outcome: string(result.Outcome),
			Blocked: result.Outcome == specgraph.UpdateOutcomeBlocked,
			Gate:    result.GateReport,
		}, nil
	}
}

func deleteEntityTool() mcp.Tool {
	return mcp.NewTool("delete_entity",
		mcp.WithDescription("Remove an entity. Refused while any relation still references it; deprecating via update_entity preserves history instead."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Entity ID to delete"),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
	)
}

func handleDeleteEntity(engine *specgraph.Engine) func(context.Context, entityIDArgs) (any, error) {
	return func(ctx context.Context, args entityIDArgs) (any, error) {
		if err := engine.DeleteEntity(ctx, args.ID); err != nil {
			return nil, err
		}
		return jsoncontract.DeleteResponse{Deleted: args.ID}, nil
	}
}

type deleteRelationArgs struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

func deleteRelationTool() mcp.Tool {
	return mcp.NewTool("delete_relation",
		mcp.WithDescription("Remove one relation between two entities"),
		mcp.WithString("from",
			mcp.Required(),
			mcp.Description("Source entity ID"),
		),
		mcp.WithString("to",
			mcp.Required(),
			mcp.Description("Target entity ID"),
		),
		mcp.WithString("type",
			mcp.Required(),
			mcp.Description("Relation type"),
			mcp.Enum(relationTypeNames()...),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
	)
}

func handleDeleteRelation(engine *specgraph.Engine) func(context.Context, deleteRelationArgs) (any, error) {
	return func(ctx context.Context, args deleteRelationArgs) (any, error) {
		request := specgraph.DeleteRelationRequest{From: args.From, To: args.To, Type: args.Type}
		if err := engine.DeleteRelation(ctx, request); err != nil {
			return nil, err
		}
		return jsoncontract.DeleteResponse{Deleted: fmt.Sprintf("%s->%s[%s]", args.From, args.To, args.Type)}, nil
	}
}

type nextPhaseArgs struct {
	Activate bool `json:"activate"`
}

func nextPhaseTool() mcp.Tool {
	return mcp.NewTool("next_phase",
		mcp.WithDescription("Find the next eligible phase in the active plan: the lowest-order phase that is unresolved and whose predecessors are all resolved. Set activate to move it from draft to active before work starts."),
		mcp.WithBoolean("activate",
			mcp.Description("Transition the selected phase from draft to active"),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
	)
}

func handleNextPhase(engine *specgraph.Engine) func(context.Context, nextPhaseArgs) (any, error) {
	return func(ctx context.Context, args nextPhaseArgs) (any, error) {
		return engine.PhaseNext(ctx, specgraph.PhaseNextRequest{Activate: args.Activate})
	}
}

func layerProperty() mcp.ToolOption {
	return mcp.WithString("layer",
		mcp.Description("Restrict to one layer. Omit for all layers."),
		mcp.Enum(layerNames()...),
	)
}

func layerNames() []string {
	names := make([]string, 0, len(model.ValidLayers))
	for _, layer := range model.ValidLayers {
		names = append(names, string(layer))
	}
	return names
}

// entityTypeNames sorts its result because ValidEntityTypes is built by ranging
// over a map, and an unstable enum would reorder the published schema per run.
func entityTypeNames() []string {
	names := make([]string, 0, len(model.ValidEntityTypes))
	for _, entityType := range model.ValidEntityTypes {
		names = append(names, string(entityType))
	}
	slices.Sort(names)
	return names
}

func relationTypeNames() []string {
	names := make([]string, 0, len(model.ValidRelationTypes))
	for _, relationType := range model.ValidRelationTypes {
		names = append(names, string(relationType))
	}
	return names
}

// statusNames lists every status the model recognizes, so the schema tracks the
// model rather than a copy of it. Narrowing per entity type stays with the
// engine, which rejects a status the given type disallows.
func statusNames() []string {
	statuses := model.ValidEntityStatuses()
	names := make([]string, 0, len(statuses))
	for _, status := range statuses {
		names = append(names, string(status))
	}
	return names
}
