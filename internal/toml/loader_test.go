package spectoml

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tyeongkim/spec-graph/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	root := filepath.Join(t.TempDir(), ".spec-graph")
	s := NewStore(root)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return s
}

func sampleEntityFile() *EntityFile {
	return &EntityFile{
		Schema:      1,
		ID:          "REQ-001",
		Type:        model.EntityTypeRequirement,
		Title:       "User authentication",
		Description: "Users must authenticate via JWT",
		Status:      model.EntityStatusActive,
		Metadata:    map[string]any{"priority": "high"},
		Relations: []RelationEntry{
			{To: "DEC-001", Type: model.RelationDependsOn},
		},
	}
}

func TestStore_Init(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".spec-graph")
	s := NewStore(root)

	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	for et := range model.TypePrefixMap {
		dir := filepath.Join(root, "entities", string(et))
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", dir)
		}
	}

	histDir := filepath.Join(root, "history")
	info, err := os.Stat(histDir)
	if err != nil {
		t.Fatalf("expected history dir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected history to be a directory")
	}
}

func TestStore_WriteAndReadEntity(t *testing.T) {
	s := newTestStore(t)
	ef := sampleEntityFile()

	if err := s.WriteEntity(ef); err != nil {
		t.Fatalf("WriteEntity: %v", err)
	}

	got, err := s.ReadEntity("REQ-001", model.EntityTypeRequirement)
	if err != nil {
		t.Fatalf("ReadEntity: %v", err)
	}

	if got.ID != ef.ID {
		t.Errorf("ID = %q, want %q", got.ID, ef.ID)
	}
	if got.Type != ef.Type {
		t.Errorf("Type = %q, want %q", got.Type, ef.Type)
	}
	if got.Title != ef.Title {
		t.Errorf("Title = %q, want %q", got.Title, ef.Title)
	}
	if got.Description != ef.Description {
		t.Errorf("Description = %q, want %q", got.Description, ef.Description)
	}
	if got.Status != ef.Status {
		t.Errorf("Status = %q, want %q", got.Status, ef.Status)
	}
	if len(got.Relations) != 1 {
		t.Fatalf("Relations len = %d, want 1", len(got.Relations))
	}
	if got.Relations[0].To != "DEC-001" {
		t.Errorf("Relations[0].To = %q, want %q", got.Relations[0].To, "DEC-001")
	}
	if got.Relations[0].Type != model.RelationDependsOn {
		t.Errorf("Relations[0].Type = %q, want %q", got.Relations[0].Type, model.RelationDependsOn)
	}
}

func TestStore_EntityExists(t *testing.T) {
	s := newTestStore(t)

	if s.EntityExists("REQ-001", model.EntityTypeRequirement) {
		t.Error("EntityExists should return false before write")
	}

	ef := sampleEntityFile()
	if err := s.WriteEntity(ef); err != nil {
		t.Fatalf("WriteEntity: %v", err)
	}

	if !s.EntityExists("REQ-001", model.EntityTypeRequirement) {
		t.Error("EntityExists should return true after write")
	}
}

func TestStore_DeleteEntity(t *testing.T) {
	s := newTestStore(t)
	ef := sampleEntityFile()

	if err := s.WriteEntity(ef); err != nil {
		t.Fatalf("WriteEntity: %v", err)
	}

	if err := s.DeleteEntity("REQ-001", model.EntityTypeRequirement); err != nil {
		t.Fatalf("DeleteEntity: %v", err)
	}

	if s.EntityExists("REQ-001", model.EntityTypeRequirement) {
		t.Error("entity should not exist after delete")
	}
}

func TestStore_ListEntities(t *testing.T) {
	s := newTestStore(t)

	entities := []*EntityFile{
		{Schema: 1, ID: "REQ-001", Type: model.EntityTypeRequirement, Title: "Req 1", Status: model.EntityStatusDraft},
		{Schema: 1, ID: "REQ-002", Type: model.EntityTypeRequirement, Title: "Req 2", Status: model.EntityStatusActive},
		{Schema: 1, ID: "DEC-001", Type: model.EntityTypeDecision, Title: "Dec 1", Status: model.EntityStatusDraft},
		{Schema: 1, ID: "PHS-001", Type: model.EntityTypePhase, Title: "Phase 1", Status: model.EntityStatusActive},
	}

	for _, ef := range entities {
		if err := s.WriteEntity(ef); err != nil {
			t.Fatalf("WriteEntity %s: %v", ef.ID, err)
		}
	}

	got, err := s.ListEntities()
	if err != nil {
		t.Fatalf("ListEntities: %v", err)
	}

	if len(got) != 4 {
		t.Fatalf("ListEntities returned %d, want 4", len(got))
	}

	ids := make(map[string]bool)
	for _, ef := range got {
		ids[ef.ID] = true
	}
	for _, ef := range entities {
		if !ids[ef.ID] {
			t.Errorf("ListEntities missing %s", ef.ID)
		}
	}
}

func TestStore_AppendHistory_CreatesFile(t *testing.T) {
	s := newTestStore(t)

	entry := HistoryEntry{
		Action:    model.ActionCreate,
		Reason:    "initial creation",
		Actor:     "agent",
		Timestamp: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
	}

	if err := s.AppendHistory("REQ-001", entry); err != nil {
		t.Fatalf("AppendHistory: %v", err)
	}

	hf, err := s.ReadHistory("REQ-001")
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}

	if hf.EntityID != "REQ-001" {
		t.Errorf("EntityID = %q, want %q", hf.EntityID, "REQ-001")
	}
	if len(hf.Entries) != 1 {
		t.Fatalf("Entries len = %d, want 1", len(hf.Entries))
	}
	if hf.Entries[0].Action != model.ActionCreate {
		t.Errorf("Action = %q, want %q", hf.Entries[0].Action, model.ActionCreate)
	}
}

func TestStore_AppendHistory_AppendsToExisting(t *testing.T) {
	s := newTestStore(t)

	entry1 := HistoryEntry{
		Action:    model.ActionCreate,
		Reason:    "initial",
		Actor:     "agent",
		Timestamp: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
	}
	entry2 := HistoryEntry{
		Action:    model.ActionUpdate,
		Reason:    "status change",
		Actor:     "agent",
		Detail:    "draft -> active",
		Timestamp: time.Date(2025, 1, 16, 12, 0, 0, 0, time.UTC),
	}

	if err := s.AppendHistory("REQ-001", entry1); err != nil {
		t.Fatalf("AppendHistory 1: %v", err)
	}
	if err := s.AppendHistory("REQ-001", entry2); err != nil {
		t.Fatalf("AppendHistory 2: %v", err)
	}

	hf, err := s.ReadHistory("REQ-001")
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}

	if len(hf.Entries) != 2 {
		t.Fatalf("Entries len = %d, want 2", len(hf.Entries))
	}
	if hf.Entries[1].Action != model.ActionUpdate {
		t.Errorf("Entries[1].Action = %q, want %q", hf.Entries[1].Action, model.ActionUpdate)
	}
	if hf.Entries[1].Detail != "draft -> active" {
		t.Errorf("Entries[1].Detail = %q, want %q", hf.Entries[1].Detail, "draft -> active")
	}
}

func TestStore_DeleteHistory(t *testing.T) {
	s := newTestStore(t)

	entry := HistoryEntry{
		Action:    model.ActionCreate,
		Reason:    "test",
		Actor:     "agent",
		Timestamp: time.Now(),
	}
	if err := s.AppendHistory("REQ-001", entry); err != nil {
		t.Fatalf("AppendHistory: %v", err)
	}

	if err := s.DeleteHistory("REQ-001"); err != nil {
		t.Fatalf("DeleteHistory: %v", err)
	}

	_, err := s.ReadHistory("REQ-001")
	if err == nil {
		t.Error("ReadHistory should fail after delete")
	}
}

func TestStore_SymmetricRelation_Enforced(t *testing.T) {
	s := newTestStore(t)

	tests := []struct {
		name     string
		ef       *EntityFile
		wantErr  bool
	}{
		{
			name: "conflicts_with stored in smaller ID - ok",
			ef: &EntityFile{
				Schema: 1, ID: "DEC-001", Type: model.EntityTypeDecision,
				Title: "d1", Status: model.EntityStatusDraft,
				Relations: []RelationEntry{
					{To: "DEC-002", Type: model.RelationConflictsWith},
				},
			},
			wantErr: false,
		},
		{
			name: "conflicts_with stored in larger ID - error",
			ef: &EntityFile{
				Schema: 1, ID: "DEC-002", Type: model.EntityTypeDecision,
				Title: "d2", Status: model.EntityStatusDraft,
				Relations: []RelationEntry{
					{To: "DEC-001", Type: model.RelationConflictsWith},
				},
			},
			wantErr: true,
		},
		{
			name: "supersedes stored in smaller ID - ok",
			ef: &EntityFile{
				Schema: 1, ID: "REQ-001", Type: model.EntityTypeRequirement,
				Title: "r1", Status: model.EntityStatusDraft,
				Relations: []RelationEntry{
					{To: "REQ-002", Type: model.RelationSupersedes},
				},
			},
			wantErr: false,
		},
		{
			name: "supersedes stored in larger ID - ok (directional, not symmetric)",
			ef: &EntityFile{
				Schema: 1, ID: "REQ-002", Type: model.EntityTypeRequirement,
				Title: "r2", Status: model.EntityStatusDraft,
				Relations: []RelationEntry{
					{To: "REQ-001", Type: model.RelationSupersedes},
				},
			},
			wantErr: false,
		},
		{
			name: "non-symmetric relation in larger ID - ok",
			ef: &EntityFile{
				Schema: 1, ID: "REQ-002", Type: model.EntityTypeRequirement,
				Title: "r2", Status: model.EntityStatusDraft,
				Relations: []RelationEntry{
					{To: "DEC-001", Type: model.RelationDependsOn},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.WriteEntity(tt.ef)
			if (err != nil) != tt.wantErr {
				t.Errorf("WriteEntity() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStore_ReadEntity_MismatchedID(t *testing.T) {
	s := newTestStore(t)

	ef := &EntityFile{
		Schema: 1, ID: "REQ-001", Type: model.EntityTypeRequirement,
		Title: "test", Status: model.EntityStatusDraft,
	}
	if err := s.WriteEntity(ef); err != nil {
		t.Fatalf("WriteEntity: %v", err)
	}

	path := s.EntityPath("REQ-001", model.EntityTypeRequirement)
	badContent := MarshalEntityFile(EntityFile{
		Schema: 1, ID: "REQ-999", Type: model.EntityTypeRequirement,
		Title: "bad", Status: model.EntityStatusDraft,
	})
	if err := os.WriteFile(path, []byte(badContent), 0o644); err != nil {
		t.Fatalf("write bad content: %v", err)
	}

	_, err := s.ReadEntity("REQ-001", model.EntityTypeRequirement)
	if err == nil {
		t.Error("ReadEntity should fail when content ID mismatches filename")
	}
}

func TestStore_ReadEntity_MismatchedType(t *testing.T) {
	s := newTestStore(t)

	ef := &EntityFile{
		Schema: 1, ID: "REQ-001", Type: model.EntityTypeRequirement,
		Title: "test", Status: model.EntityStatusDraft,
	}
	if err := s.WriteEntity(ef); err != nil {
		t.Fatalf("WriteEntity: %v", err)
	}

	path := s.EntityPath("REQ-001", model.EntityTypeRequirement)
	badContent := MarshalEntityFile(EntityFile{
		Schema: 1, ID: "REQ-001", Type: model.EntityTypeDecision,
		Title: "bad", Status: model.EntityStatusDraft,
	})
	if err := os.WriteFile(path, []byte(badContent), 0o644); err != nil {
		t.Fatalf("write bad content: %v", err)
	}

	_, err := s.ReadEntity("REQ-001", model.EntityTypeRequirement)
	if err == nil {
		t.Error("ReadEntity should fail when content type mismatches directory")
	}
}

func TestStore_AtomicWrite_ConcurrentAccess(t *testing.T) {
	s := newTestStore(t)

	ef := &EntityFile{
		Schema: 1, ID: "REQ-001", Type: model.EntityTypeRequirement,
		Title: "concurrent", Status: model.EntityStatusDraft,
	}

	var wg sync.WaitGroup
	errs := make([]error, 20)

	for i := range 20 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			localEF := *ef
			localEF.Title = fmt.Sprintf("version-%d", idx)
			errs[idx] = s.WriteEntity(&localEF)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: WriteEntity error: %v", i, err)
		}
	}

	got, err := s.ReadEntity("REQ-001", model.EntityTypeRequirement)
	if err != nil {
		t.Fatalf("ReadEntity after concurrent writes: %v", err)
	}
	if got.ID != "REQ-001" {
		t.Errorf("ID = %q after concurrent writes, want REQ-001", got.ID)
	}
}

func TestStore_EntityPath(t *testing.T) {
	s := NewStore("/tmp/.spec-graph")

	got := s.EntityPath("REQ-001", model.EntityTypeRequirement)
	want := "/tmp/.spec-graph/entities/requirement/REQ-001.toml"
	if got != want {
		t.Errorf("EntityPath = %q, want %q", got, want)
	}
}

func TestStore_HistoryPath(t *testing.T) {
	s := NewStore("/tmp/.spec-graph")

	got := s.HistoryPath("REQ-001")
	want := "/tmp/.spec-graph/history/REQ-001.toml"
	if got != want {
		t.Errorf("HistoryPath = %q, want %q", got, want)
	}
}
