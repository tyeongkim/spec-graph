package spectoml

import (
	"path/filepath"
	"testing"
)

func TestLoadSchema(t *testing.T) {
	path := filepath.Join("testdata", "schema.toml")
	s, err := LoadSchema(path)
	if err != nil {
		t.Fatalf("LoadSchema(%q) error: %v", path, err)
	}

	if s.Version != 1 {
		t.Errorf("Version = %d; want 1", s.Version)
	}
	if len(s.EntityTypes) != 12 {
		t.Errorf("len(EntityTypes) = %d; want 12", len(s.EntityTypes))
	}
	if len(s.RelationTypes) != 17 {
		t.Errorf("len(RelationTypes) = %d; want 17", len(s.RelationTypes))
	}
}

func TestLoadSchema_FileNotFound(t *testing.T) {
	_, err := LoadSchema("testdata/nonexistent.toml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestParseSchema_InvalidTOML(t *testing.T) {
	_, err := ParseSchema([]byte(`[[[invalid`))
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestParseSchema_ValidationErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "unsupported version",
			input: `version = 99` + "\n[entity_types.x]\nprefix = \"X\"\nlayer = \"arch\"\nallowed_status = [\"draft\"]\n[relation_types.r]\nlayer = \"arch\"\nspecial = \"any_to_any\"\n",
		},
		{
			name:  "no entity types",
			input: "version = 1\n[relation_types.r]\nlayer = \"arch\"\nspecial = \"any_to_any\"\n",
		},
		{
			name:  "no relation types",
			input: "version = 1\n[entity_types.x]\nprefix = \"X\"\nlayer = \"arch\"\nallowed_status = [\"draft\"]\n",
		},
		{
			name:  "empty prefix",
			input: "version = 1\n[entity_types.x]\nprefix = \"\"\nlayer = \"arch\"\nallowed_status = [\"draft\"]\n[relation_types.r]\nlayer = \"arch\"\nspecial = \"any_to_any\"\n",
		},
		{
			name:  "duplicate prefix",
			input: "version = 1\n[entity_types.x]\nprefix = \"DUP\"\nlayer = \"arch\"\nallowed_status = [\"draft\"]\n[entity_types.y]\nprefix = \"DUP\"\nlayer = \"arch\"\nallowed_status = [\"draft\"]\n[relation_types.r]\nlayer = \"arch\"\nspecial = \"any_to_any\"\n",
		},
		{
			name:  "invalid entity layer",
			input: "version = 1\n[entity_types.x]\nprefix = \"X\"\nlayer = \"bad\"\nallowed_status = [\"draft\"]\n[relation_types.r]\nlayer = \"arch\"\nspecial = \"any_to_any\"\n",
		},
		{
			name:  "empty allowed_status",
			input: "version = 1\n[entity_types.x]\nprefix = \"X\"\nlayer = \"arch\"\nallowed_status = []\n[relation_types.r]\nlayer = \"arch\"\nspecial = \"any_to_any\"\n",
		},
		{
			name:  "invalid relation layer",
			input: "version = 1\n[entity_types.x]\nprefix = \"X\"\nlayer = \"arch\"\nallowed_status = [\"draft\"]\n[relation_types.r]\nlayer = \"bad\"\nspecial = \"any_to_any\"\n",
		},
		{
			name:  "invalid special value",
			input: "version = 1\n[entity_types.x]\nprefix = \"X\"\nlayer = \"arch\"\nallowed_status = [\"draft\"]\n[relation_types.r]\nlayer = \"arch\"\nspecial = \"invalid\"\n",
		},
		{
			name:  "relation empty from",
			input: "version = 1\n[entity_types.x]\nprefix = \"X\"\nlayer = \"arch\"\nallowed_status = [\"draft\"]\n[relation_types.r]\nlayer = \"arch\"\nfrom = []\nto = [\"x\"]\n",
		},
		{
			name:  "relation empty to",
			input: "version = 1\n[entity_types.x]\nprefix = \"X\"\nlayer = \"arch\"\nallowed_status = [\"draft\"]\n[relation_types.r]\nlayer = \"arch\"\nfrom = [\"x\"]\nto = []\n",
		},
		{
			name:  "relation unknown from entity",
			input: "version = 1\n[entity_types.x]\nprefix = \"X\"\nlayer = \"arch\"\nallowed_status = [\"draft\"]\n[relation_types.r]\nlayer = \"arch\"\nfrom = [\"unknown\"]\nto = [\"x\"]\n",
		},
		{
			name:  "relation unknown to entity",
			input: "version = 1\n[entity_types.x]\nprefix = \"X\"\nlayer = \"arch\"\nallowed_status = [\"draft\"]\n[relation_types.r]\nlayer = \"arch\"\nfrom = [\"x\"]\nto = [\"unknown\"]\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseSchema([]byte(tc.input))
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestDefaultSchema_Validate(t *testing.T) {
	s := DefaultSchema()
	if err := s.validate(); err != nil {
		t.Fatalf("DefaultSchema() fails validation: %v", err)
	}
}

func TestValidateEntity(t *testing.T) {
	s := DefaultSchema()

	tests := []struct {
		name       string
		id         string
		entityType string
		status     string
		wantErr    bool
	}{
		{"valid requirement", "REQ-001", "requirement", "draft", false},
		{"valid decision active", "DEC-042", "decision", "active", false},
		{"valid phase deprecated", "PHS-100", "phase", "deprecated", false},
		{"valid question resolved", "QST-001", "question", "resolved", false},
		{"unknown entity type", "FOO-001", "unknown", "draft", true},
		{"wrong prefix", "DEC-001", "requirement", "draft", true},
		{"invalid ID format", "req001", "requirement", "draft", true},
		{"empty ID", "", "requirement", "draft", true},
		{"invalid status for question", "QST-001", "question", "deprecated", true},
		{"invalid status value", "REQ-001", "requirement", "invalid", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := s.ValidateEntity(tc.id, tc.entityType, tc.status)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateEntity(%q, %q, %q) error = %v; wantErr = %v",
					tc.id, tc.entityType, tc.status, err, tc.wantErr)
			}
		})
	}
}

func TestValidateRelation(t *testing.T) {
	s := DefaultSchema()

	tests := []struct {
		name         string
		fromType     string
		toType       string
		relationType string
		wantErr      bool
	}{
		{"implements: interface→requirement", "interface", "requirement", "implements", false},
		{"implements: interface→criterion", "interface", "criterion", "implements", false},
		{"implements: test→requirement INVALID", "test", "requirement", "implements", true},
		{"implements: interface→decision INVALID", "interface", "decision", "implements", true},

		{"verifies: test→requirement", "test", "requirement", "verifies", false},
		{"verifies: test→state", "test", "state", "verifies", false},
		{"verifies: requirement→test INVALID", "requirement", "test", "verifies", true},

		{"depends_on: requirement→decision", "requirement", "decision", "depends_on", false},
		{"depends_on: test→assumption", "test", "assumption", "depends_on", false},
		{"depends_on: plan→requirement INVALID", "plan", "requirement", "depends_on", true},

		{"constrained_by: state→crosscut", "state", "crosscut", "constrained_by", false},
		{"constrained_by: test→crosscut INVALID", "test", "crosscut", "constrained_by", true},

		{"triggers: interface→state", "interface", "state", "triggers", false},
		{"triggers: decision→state", "decision", "state", "triggers", false},
		{"triggers: test→state INVALID", "test", "state", "triggers", true},

		{"answers: decision→question", "decision", "question", "answers", false},
		{"answers: requirement→question INVALID", "requirement", "question", "answers", true},

		{"assumes: requirement→assumption", "requirement", "assumption", "assumes", false},
		{"assumes: test→assumption INVALID", "test", "assumption", "assumes", true},

		{"has_criterion: requirement→criterion", "requirement", "criterion", "has_criterion", false},
		{"has_criterion: decision→criterion INVALID", "decision", "criterion", "has_criterion", true},

		{"mitigates: decision→risk", "decision", "risk", "mitigates", false},
		{"mitigates: crosscut→risk", "crosscut", "risk", "mitigates", false},
		{"mitigates: requirement→risk INVALID", "requirement", "risk", "mitigates", true},

		{"supersedes: same type", "requirement", "requirement", "supersedes", false},
		{"supersedes: different type INVALID", "requirement", "decision", "supersedes", true},

		{"conflicts_with: any→any", "test", "risk", "conflicts_with", false},
		{"references: any→any", "risk", "phase", "references", false},

		{"belongs_to: phase→plan", "phase", "plan", "belongs_to", false},
		{"belongs_to: plan→phase INVALID", "plan", "phase", "belongs_to", true},

		{"precedes: phase→phase", "phase", "phase", "precedes", false},
		{"precedes: plan→phase INVALID", "plan", "phase", "precedes", true},

		{"blocks: phase→phase", "phase", "phase", "blocks", false},
		{"blocks: phase→requirement INVALID", "phase", "requirement", "blocks", true},

		{"covers: phase→requirement", "phase", "requirement", "covers", false},
		{"covers: phase→assumption", "phase", "assumption", "covers", false},
		{"covers: phase→phase INVALID", "phase", "phase", "covers", true},
		{"covers: requirement→decision INVALID", "requirement", "decision", "covers", true},

		{"delivers: phase→requirement", "phase", "requirement", "delivers", false},
		{"delivers: phase→criterion", "phase", "criterion", "delivers", false},
		{"delivers: phase→question INVALID", "phase", "question", "delivers", true},

		{"unknown relation type", "requirement", "decision", "unknown", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := s.ValidateRelation(tc.fromType, tc.toType, tc.relationType)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateRelation(%q, %q, %q) error = %v; wantErr = %v",
					tc.fromType, tc.toType, tc.relationType, err, tc.wantErr)
			}
		})
	}
}

func TestIsRelationAllowed_MatchesEdgeMatrix(t *testing.T) {
	s := DefaultSchema()

	type edgeCase struct {
		from     string
		to       string
		relation string
		want     bool
	}

	cases := []edgeCase{
		{"interface", "requirement", "implements", true},
		{"interface", "criterion", "implements", true},
		{"test", "requirement", "implements", false},

		{"test", "requirement", "verifies", true},
		{"test", "criterion", "verifies", true},
		{"test", "decision", "verifies", true},
		{"test", "interface", "verifies", true},
		{"test", "state", "verifies", true},
		{"requirement", "test", "verifies", false},

		{"requirement", "requirement", "depends_on", true},
		{"requirement", "decision", "depends_on", true},
		{"decision", "interface", "depends_on", true},
		{"interface", "state", "depends_on", true},
		{"test", "crosscut", "depends_on", true},
		{"state", "assumption", "depends_on", true},
		{"plan", "requirement", "depends_on", false},

		{"requirement", "crosscut", "constrained_by", true},
		{"decision", "decision", "constrained_by", true},
		{"interface", "assumption", "constrained_by", true},
		{"state", "crosscut", "constrained_by", true},
		{"test", "crosscut", "constrained_by", false},

		{"interface", "state", "triggers", true},
		{"decision", "state", "triggers", true},
		{"test", "state", "triggers", false},

		{"decision", "question", "answers", true},
		{"requirement", "question", "answers", false},

		{"requirement", "assumption", "assumes", true},
		{"decision", "assumption", "assumes", true},
		{"interface", "assumption", "assumes", true},
		{"test", "assumption", "assumes", false},

		{"requirement", "criterion", "has_criterion", true},
		{"decision", "criterion", "has_criterion", false},

		{"decision", "risk", "mitigates", true},
		{"test", "risk", "mitigates", true},
		{"crosscut", "risk", "mitigates", true},
		{"requirement", "risk", "mitigates", false},

		{"requirement", "requirement", "supersedes", true},
		{"decision", "decision", "supersedes", true},
		{"requirement", "decision", "supersedes", false},

		{"requirement", "decision", "conflicts_with", true},
		{"test", "risk", "conflicts_with", true},

		{"requirement", "phase", "references", true},
		{"risk", "plan", "references", true},

		{"phase", "plan", "belongs_to", true},
		{"plan", "phase", "belongs_to", false},

		{"phase", "phase", "precedes", true},
		{"plan", "phase", "precedes", false},

		{"phase", "phase", "blocks", true},
		{"plan", "phase", "blocks", false},

		{"phase", "requirement", "covers", true},
		{"phase", "decision", "covers", true},
		{"phase", "interface", "covers", true},
		{"phase", "test", "covers", true},
		{"phase", "question", "covers", true},
		{"phase", "risk", "covers", true},
		{"phase", "criterion", "covers", true},
		{"phase", "assumption", "covers", true},
		{"phase", "phase", "covers", false},
		{"requirement", "decision", "covers", false},

		{"phase", "requirement", "delivers", true},
		{"phase", "interface", "delivers", true},
		{"phase", "state", "delivers", true},
		{"phase", "test", "delivers", true},
		{"phase", "decision", "delivers", true},
		{"phase", "criterion", "delivers", true},
		{"phase", "question", "delivers", false},
		{"requirement", "interface", "delivers", false},
	}

	for _, tc := range cases {
		t.Run(tc.relation+"/"+tc.from+"→"+tc.to, func(t *testing.T) {
			got := s.IsRelationAllowed(tc.from, tc.to, tc.relation)
			if got != tc.want {
				t.Errorf("IsRelationAllowed(%q, %q, %q) = %v; want %v",
					tc.from, tc.to, tc.relation, got, tc.want)
			}
		})
	}
}

func TestLoadedSchema_MatchesDefault(t *testing.T) {
	loaded, err := LoadSchema(filepath.Join("testdata", "schema.toml"))
	if err != nil {
		t.Fatalf("LoadSchema error: %v", err)
	}

	def := DefaultSchema()

	if loaded.Version != def.Version {
		t.Errorf("Version mismatch: loaded=%d, default=%d", loaded.Version, def.Version)
	}

	for name, defCfg := range def.EntityTypes {
		loadedCfg, ok := loaded.EntityTypes[name]
		if !ok {
			t.Errorf("entity type %q missing from loaded schema", name)
			continue
		}
		if loadedCfg.Prefix != defCfg.Prefix {
			t.Errorf("entity %q prefix: loaded=%q, default=%q", name, loadedCfg.Prefix, defCfg.Prefix)
		}
		if loadedCfg.Layer != defCfg.Layer {
			t.Errorf("entity %q layer: loaded=%q, default=%q", name, loadedCfg.Layer, defCfg.Layer)
		}
	}

	for name, defCfg := range def.RelationTypes {
		loadedCfg, ok := loaded.RelationTypes[name]
		if !ok {
			t.Errorf("relation type %q missing from loaded schema", name)
			continue
		}
		if loadedCfg.Layer != defCfg.Layer {
			t.Errorf("relation %q layer: loaded=%q, default=%q", name, loadedCfg.Layer, defCfg.Layer)
		}
		if loadedCfg.Special != defCfg.Special {
			t.Errorf("relation %q special: loaded=%q, default=%q", name, loadedCfg.Special, defCfg.Special)
		}
	}
}
