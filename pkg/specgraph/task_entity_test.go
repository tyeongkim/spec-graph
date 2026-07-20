package specgraph_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

func TestTaskContractRoundTrip(t *testing.T) {
	root, engine := openTaskTestEngine(t)
	want := validTaskContract()
	metadata := marshalTaskContract(t, want)

	created, err := engine.CreateEntity(context.Background(), specgraph.CreateEntityRequest{
		Type:        "task",
		ID:          "TSK-001",
		Title:       "Implement task contract",
		Description: "Add strict task metadata decoding.",
		Metadata:    metadata,
	})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}
	if created.Layer != model.LayerExec {
		t.Fatalf("created layer = %q; want %q", created.Layer, model.LayerExec)
	}

	readBack, err := engine.GetEntity(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	got, err := model.DecodeTaskContract(readBack.Metadata, readBack.Status)
	if err != nil {
		t.Fatalf("DecodeTaskContract: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip contract = %#v; want %#v", got, want)
	}
	if _, err := os.ReadFile(taskEntityPath(root, created.ID)); err != nil {
		t.Fatalf("read persisted task TOML: %v", err)
	}
}

func TestTaskLifecycleValidTransitions(t *testing.T) {
	_, engine := openTaskTestEngine(t)
	createTask(t, engine, "TSK-001")

	active := string(model.EntityStatusActive)
	result, err := engine.UpdateEntity(context.Background(), specgraph.UpdateEntityRequest{ID: "TSK-001", Status: &active})
	if err != nil {
		t.Fatalf("draft to active: %v", err)
	}
	if result.Entity.Status != model.EntityStatusActive {
		t.Fatalf("status = %q; want active", result.Entity.Status)
	}

	contract := validTaskContract()
	contract.QA[0].Evidence = "PASS: go test ./pkg/specgraph"
	metadata := marshalTaskContract(t, contract)
	resolved := string(model.EntityStatusResolved)
	result, err = engine.UpdateEntity(context.Background(), specgraph.UpdateEntityRequest{
		ID:       "TSK-001",
		Status:   &resolved,
		Metadata: &metadata,
	})
	if err != nil {
		t.Fatalf("active to resolved: %v", err)
	}
	if result.Entity.Status != model.EntityStatusResolved {
		t.Fatalf("status = %q; want resolved", result.Entity.Status)
	}
}

func TestTaskContractRejectsInvalidShapes(t *testing.T) {
	tests := []struct {
		name        string
		metadata    string
		title       string
		description string
	}{
		{"order zero", taskMetadata(`"order":0`), "Valid title", "Valid description"},
		{"order negative", taskMetadata(`"order":-1`), "Valid title", "Valid description"},
		{"order fractional", taskMetadata(`"order":1.5`), "Valid title", "Valid description"},
		{"missing instructions", `{"order":1,"acceptance":["done"],"must_not":[],"references":[],"qa":[{"command":"go test","expected":"pass","evidence":""}]}`, "Valid title", "Valid description"},
		{"empty instructions", taskMetadata(`"instructions":[]`), "Valid title", "Valid description"},
		{"missing acceptance", `{"order":1,"instructions":["work"],"must_not":[],"references":[],"qa":[{"command":"go test","expected":"pass","evidence":""}]}`, "Valid title", "Valid description"},
		{"empty acceptance", taskMetadata(`"acceptance":[]`), "Valid title", "Valid description"},
		{"missing must_not", `{"order":1,"instructions":["work"],"acceptance":["done"],"references":[],"qa":[{"command":"go test","expected":"pass","evidence":""}]}`, "Valid title", "Valid description"},
		{"missing references", `{"order":1,"instructions":["work"],"acceptance":["done"],"must_not":[],"qa":[{"command":"go test","expected":"pass","evidence":""}]}`, "Valid title", "Valid description"},
		{"empty instruction element", taskMetadata(`"instructions":[" "]`), "Valid title", "Valid description"},
		{"empty acceptance element", taskMetadata(`"acceptance":[""]`), "Valid title", "Valid description"},
		{"empty must_not element", taskMetadata(`"must_not":[""]`), "Valid title", "Valid description"},
		{"empty reference element", taskMetadata(`"references":[" "]`), "Valid title", "Valid description"},
		{"unknown metadata key", taskMetadata(`"agent":"forbidden"`), "Valid title", "Valid description"},
		{"empty title", taskMetadata(""), " ", "Valid description"},
		{"empty description", taskMetadata(""), "Valid title", "\t"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, engine := openTaskTestEngine(t)
			createTask(t, engine, "TSK-001")
			path := taskEntityPath(root, "TSK-001")
			before := readTaskBytes(t, path)

			metadata := json.RawMessage(test.metadata)
			_, err := engine.UpdateEntity(context.Background(), specgraph.UpdateEntityRequest{
				ID:          "TSK-001",
				Title:       &test.title,
				Description: &test.description,
				Metadata:    &metadata,
			})
			assertErrorCode(t, err, specgraph.CodeInvalidInput)
			after := readTaskBytes(t, path)
			if !reflect.DeepEqual(after, before) {
				t.Fatal("task TOML changed after rejected update")
			}
		})
	}
}

func TestTaskLifecycleRejectsTerminalReopen(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(map[bool]string{false: "without force", true: "with force"}[force], func(t *testing.T) {
			root, engine := openTaskTestEngine(t)
			resolveTask(t, engine, "TSK-001")
			path := taskEntityPath(root, "TSK-001")
			before := readTaskBytes(t, path)
			active := string(model.EntityStatusActive)

			_, err := engine.UpdateEntity(context.Background(), specgraph.UpdateEntityRequest{
				ID: "TSK-001", Status: &active, Force: force,
			})
			assertErrorCode(t, err, specgraph.CodeInvalidInput)
			if after := readTaskBytes(t, path); !reflect.DeepEqual(after, before) {
				t.Fatal("task TOML changed after rejected terminal reopen")
			}
		})
	}
}

func TestTaskDeprecationRequiresReason(t *testing.T) {
	tests := []struct {
		name      string
		deprecate func(context.Context, *specgraph.Engine) error
	}{
		{
			name: "deprecate API",
			deprecate: func(ctx context.Context, engine *specgraph.Engine) error {
				_, err := engine.DeprecateEntity(ctx, "TSK-001", "")
				return err
			},
		},
		{
			name: "status update",
			deprecate: func(ctx context.Context, engine *specgraph.Engine) error {
				deprecated := string(model.EntityStatusDeprecated)
				_, err := engine.UpdateEntity(ctx, specgraph.UpdateEntityRequest{ID: "TSK-001", Status: &deprecated})
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, engine := openTaskTestEngine(t)
			createTask(t, engine, "TSK-001")
			path := taskEntityPath(root, "TSK-001")
			before := readTaskBytes(t, path)
			assertErrorCode(t, test.deprecate(context.Background(), engine), specgraph.CodeInvalidInput)
			if after := readTaskBytes(t, path); !reflect.DeepEqual(after, before) {
				t.Fatal("task TOML changed after reasonless deprecation")
			}
		})
	}
}

func openTaskTestEngine(t *testing.T) (string, *specgraph.Engine) {
	t.Helper()
	root := newInitializedRoot(t)
	engine, err := specgraph.Open(context.Background(), specgraph.Options{Root: root})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	return root, engine
}

func createTask(t *testing.T, engine *specgraph.Engine, id string) {
	t.Helper()
	_, err := engine.CreateEntity(context.Background(), specgraph.CreateEntityRequest{
		Type: "task", ID: id, Title: "Valid task", Description: "Valid task description.",
		Metadata: marshalTaskContract(t, validTaskContract()),
	})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}
}

func resolveTask(t *testing.T, engine *specgraph.Engine, id string) {
	t.Helper()
	createTask(t, engine, id)
	active := string(model.EntityStatusActive)
	if _, err := engine.UpdateEntity(context.Background(), specgraph.UpdateEntityRequest{ID: id, Status: &active}); err != nil {
		t.Fatalf("activate task: %v", err)
	}
	contract := validTaskContract()
	contract.QA[0].Evidence = "PASS"
	metadata := marshalTaskContract(t, contract)
	resolved := string(model.EntityStatusResolved)
	if _, err := engine.UpdateEntity(context.Background(), specgraph.UpdateEntityRequest{ID: id, Status: &resolved, Metadata: &metadata}); err != nil {
		t.Fatalf("resolve task: %v", err)
	}
}

func validTaskContract() model.TaskContract {
	return model.TaskContract{
		Order: 2, Instructions: []string{"Implement the parser", "Wire the engine"},
		Acceptance: []string{"Round-trip succeeds"}, MustNot: []string{}, References: []string{"REQ-001"},
		QA: []model.QAItem{{Command: "go test ./pkg/specgraph", Expected: "tests pass", Evidence: ""}},
	}
}

func marshalTaskContract(t *testing.T, contract model.TaskContract) json.RawMessage {
	t.Helper()
	metadata, err := json.Marshal(contract)
	if err != nil {
		t.Fatalf("marshal task contract: %v", err)
	}
	return metadata
}

func taskMetadata(replacement string) string {
	fields := map[string]string{
		"order":        `"order":1`,
		"instructions": `"instructions":["work"]`,
		"acceptance":   `"acceptance":["done"]`,
		"must_not":     `"must_not":[]`,
		"references":   `"references":[]`,
		"qa":           `"qa":[{"command":"go test","expected":"pass","evidence":""}]`,
	}
	if replacement != "" {
		var key string
		if err := json.Unmarshal([]byte("{"+replacement+"}"), &map[string]any{}); err == nil {
			for candidate := range fields {
				if len(replacement) > len(candidate)+2 && replacement[1:len(candidate)+1] == candidate {
					key = candidate
					break
				}
			}
		}
		if key == "" {
			return "{" + replacement + "," + fields["order"] + "," + fields["instructions"] + "," + fields["acceptance"] + "," + fields["must_not"] + "," + fields["references"] + "," + fields["qa"] + "}"
		}
		fields[key] = replacement
	}
	return "{" + fields["order"] + "," + fields["instructions"] + "," + fields["acceptance"] + "," + fields["must_not"] + "," + fields["references"] + "," + fields["qa"] + "}"
}

func taskEntityPath(root, id string) string {
	return filepath.Join(root, "entities", "task", id+".toml")
}

func readTaskBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read task TOML: %v", err)
	}
	return data
}
