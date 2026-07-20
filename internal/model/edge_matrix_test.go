package model

import "testing"

func TestExecEdgeMatrix(t *testing.T) {
	tests := []struct {
		name     string
		relType  RelationType
		from     EntityType
		to       EntityType
		expected bool
	}{
		// belongs_to: phase → plan
		{"belongs_to/phase→plan", RelationBelongsTo, EntityTypePhase, EntityTypePlan, true},
		{"belongs_to/plan→phase INVALID", RelationBelongsTo, EntityTypePlan, EntityTypePhase, false},
		{"belongs_to/requirement→plan INVALID", RelationBelongsTo, EntityTypeRequirement, EntityTypePlan, false},
		{"belongs_to/phase→phase INVALID", RelationBelongsTo, EntityTypePhase, EntityTypePhase, false},

		// precedes: phase → phase
		{"precedes/phase→phase", RelationPrecedes, EntityTypePhase, EntityTypePhase, true},
		{"precedes/plan→phase INVALID", RelationPrecedes, EntityTypePlan, EntityTypePhase, false},
		{"precedes/phase→plan INVALID", RelationPrecedes, EntityTypePhase, EntityTypePlan, false},
		{"precedes/requirement→phase INVALID", RelationPrecedes, EntityTypeRequirement, EntityTypePhase, false},

		// blocks: phase → phase
		{"blocks/phase→phase", RelationBlocks, EntityTypePhase, EntityTypePhase, true},
		{"blocks/plan→phase INVALID", RelationBlocks, EntityTypePlan, EntityTypePhase, false},
		{"blocks/phase→requirement INVALID", RelationBlocks, EntityTypePhase, EntityTypeRequirement, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsEdgeAllowed(tc.relType, tc.from, tc.to, nil)
			if got != tc.expected {
				t.Errorf("IsEdgeAllowed(%q, %q, %q, nil) = %v; want %v",
					tc.relType, tc.from, tc.to, got, tc.expected)
			}
		})
	}
}

func TestTaskRelationsAllowedPairs(t *testing.T) {
	tests := []struct {
		name     string
		relation RelationType
		from     EntityType
		to       EntityType
		allowed  bool
	}{
		{name: "phase belongs to plan", relation: RelationBelongsTo, from: EntityTypePhase, to: EntityTypePlan, allowed: true},
		{name: "task belongs to phase", relation: RelationBelongsTo, from: EntityTypeTask, to: EntityTypePhase, allowed: true},
		{name: "phase cannot belong to phase", relation: RelationBelongsTo, from: EntityTypePhase, to: EntityTypePhase, allowed: false},
		{name: "task cannot belong to plan", relation: RelationBelongsTo, from: EntityTypeTask, to: EntityTypePlan, allowed: false},
		{name: "task depends on task", relation: RelationTaskDependsOn, from: EntityTypeTask, to: EntityTypeTask, allowed: true},
		{name: "task covers requirement", relation: RelationCovers, from: EntityTypeTask, to: EntityTypeRequirement, allowed: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsEdgeAllowed(test.relation, test.from, test.to, nil); got != test.allowed {
				t.Errorf("IsEdgeAllowed(%q, %q, %q) = %v; want %v", test.relation, test.from, test.to, got, test.allowed)
			}
		})
	}
}

func TestMappingEdgeMatrix(t *testing.T) {
	tests := []struct {
		name     string
		relType  RelationType
		from     EntityType
		to       EntityType
		expected bool
	}{
		// covers: phase → requirement, decision, interface, test, question, risk, criterion, assumption
		{"covers/phase→requirement", RelationCovers, EntityTypePhase, EntityTypeRequirement, true},
		{"covers/phase→decision", RelationCovers, EntityTypePhase, EntityTypeDecision, true},
		{"covers/phase→interface", RelationCovers, EntityTypePhase, EntityTypeInterface, true},
		{"covers/phase→test", RelationCovers, EntityTypePhase, EntityTypeTest, true},
		{"covers/phase→question", RelationCovers, EntityTypePhase, EntityTypeQuestion, true},
		{"covers/phase→risk", RelationCovers, EntityTypePhase, EntityTypeRisk, true},
		{"covers/phase→criterion", RelationCovers, EntityTypePhase, EntityTypeCriterion, true},
		{"covers/phase→assumption", RelationCovers, EntityTypePhase, EntityTypeAssumption, true},
		{"covers/phase→phase INVALID", RelationCovers, EntityTypePhase, EntityTypePhase, false},
		{"covers/phase→plan INVALID", RelationCovers, EntityTypePhase, EntityTypePlan, false},
		{"covers/requirement→decision INVALID", RelationCovers, EntityTypeRequirement, EntityTypeDecision, false},

		// delivers: phase → requirement, interface, state, test, decision, criterion
		{"delivers/phase→requirement", RelationDelivers, EntityTypePhase, EntityTypeRequirement, true},
		{"delivers/phase→interface", RelationDelivers, EntityTypePhase, EntityTypeInterface, true},
		{"delivers/phase→state", RelationDelivers, EntityTypePhase, EntityTypeState, true},
		{"delivers/phase→test", RelationDelivers, EntityTypePhase, EntityTypeTest, true},
		{"delivers/phase→decision", RelationDelivers, EntityTypePhase, EntityTypeDecision, true},
		{"delivers/phase→criterion", RelationDelivers, EntityTypePhase, EntityTypeCriterion, true},
		{"delivers/phase→phase INVALID", RelationDelivers, EntityTypePhase, EntityTypePhase, false},
		{"delivers/phase→question INVALID", RelationDelivers, EntityTypePhase, EntityTypeQuestion, false},
		{"delivers/phase→plan INVALID", RelationDelivers, EntityTypePhase, EntityTypePlan, false},
		{"delivers/requirement→interface INVALID", RelationDelivers, EntityTypeRequirement, EntityTypeInterface, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsEdgeAllowed(tc.relType, tc.from, tc.to, nil)
			if got != tc.expected {
				t.Errorf("IsEdgeAllowed(%q, %q, %q, nil) = %v; want %v",
					tc.relType, tc.from, tc.to, got, tc.expected)
			}
		})
	}
}

func TestIsEdgeAllowed_LayerScoped(t *testing.T) {
	arch := LayerArch
	exec := LayerExec
	mapping := LayerMapping

	tests := []struct {
		name     string
		relType  RelationType
		from     EntityType
		to       EntityType
		layer    *Layer
		expected bool
	}{
		// Arch relation found only in arch matrix
		{"arch/implements in arch layer", RelationImplements, EntityTypeInterface, EntityTypeRequirement, &arch, true},
		{"arch/implements in exec layer", RelationImplements, EntityTypeInterface, EntityTypeRequirement, &exec, false},
		{"arch/implements in mapping layer", RelationImplements, EntityTypeInterface, EntityTypeRequirement, &mapping, false},

		// Exec relation found only in exec matrix
		{"exec/belongs_to in exec layer", RelationBelongsTo, EntityTypePhase, EntityTypePlan, &exec, true},
		{"exec/belongs_to in arch layer", RelationBelongsTo, EntityTypePhase, EntityTypePlan, &arch, false},
		{"exec/belongs_to in mapping layer", RelationBelongsTo, EntityTypePhase, EntityTypePlan, &mapping, false},

		{"exec/precedes in exec layer", RelationPrecedes, EntityTypePhase, EntityTypePhase, &exec, true},
		{"exec/precedes in arch layer", RelationPrecedes, EntityTypePhase, EntityTypePhase, &arch, false},

		{"exec/blocks in exec layer", RelationBlocks, EntityTypePhase, EntityTypePhase, &exec, true},
		{"exec/blocks in mapping layer", RelationBlocks, EntityTypePhase, EntityTypePhase, &mapping, false},

		// Mapping relation found only in mapping matrix
		{"mapping/covers in mapping layer", RelationCovers, EntityTypePhase, EntityTypeRequirement, &mapping, true},
		{"mapping/covers in arch layer", RelationCovers, EntityTypePhase, EntityTypeRequirement, &arch, false},
		{"mapping/covers in exec layer", RelationCovers, EntityTypePhase, EntityTypeRequirement, &exec, false},

		{"mapping/delivers in mapping layer", RelationDelivers, EntityTypePhase, EntityTypeInterface, &mapping, true},
		{"mapping/delivers in arch layer", RelationDelivers, EntityTypePhase, EntityTypeInterface, &arch, false},

		// Special cases work regardless of layer
		{"supersedes same type with arch layer", RelationSupersedes, EntityTypeRequirement, EntityTypeRequirement, &arch, true},
		{"supersedes diff type with arch layer", RelationSupersedes, EntityTypeRequirement, EntityTypeDecision, &arch, false},
		{"conflicts_with with exec layer", RelationConflictsWith, EntityTypeRequirement, EntityTypeDecision, &exec, true},
		{"references with mapping layer", RelationReferences, EntityTypeRequirement, EntityTypePhase, &mapping, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsEdgeAllowed(tc.relType, tc.from, tc.to, tc.layer)
			if got != tc.expected {
				t.Errorf("IsEdgeAllowed(%q, %q, %q, %v) = %v; want %v",
					tc.relType, tc.from, tc.to, *tc.layer, got, tc.expected)
			}
		})
	}
}

func TestIsEdgeAllowed_NilLayerBackwardCompat(t *testing.T) {
	// Verify nil layer checks all matrices — arch, exec, and mapping rules all work.
	tests := []struct {
		name     string
		relType  RelationType
		from     EntityType
		to       EntityType
		expected bool
	}{
		// Arch matrix rule
		{"nil/arch implements", RelationImplements, EntityTypeInterface, EntityTypeRequirement, true},
		// Exec matrix rule
		{"nil/exec belongs_to", RelationBelongsTo, EntityTypePhase, EntityTypePlan, true},
		{"nil/exec precedes", RelationPrecedes, EntityTypePhase, EntityTypePhase, true},
		{"nil/exec blocks", RelationBlocks, EntityTypePhase, EntityTypePhase, true},
		// Mapping matrix rule
		{"nil/mapping covers", RelationCovers, EntityTypePhase, EntityTypeRequirement, true},
		{"nil/mapping delivers", RelationDelivers, EntityTypePhase, EntityTypeInterface, true},
		// Special cases
		{"nil/supersedes same", RelationSupersedes, EntityTypeDecision, EntityTypeDecision, true},
		{"nil/supersedes diff", RelationSupersedes, EntityTypeDecision, EntityTypeRequirement, false},
		{"nil/conflicts_with", RelationConflictsWith, EntityTypeTest, EntityTypeRisk, true},
		{"nil/references", RelationReferences, EntityTypeRisk, EntityTypePhase, true},
		// Unknown relation
		{"nil/unknown relation", RelationType("unknown"), EntityTypeRequirement, EntityTypeDecision, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsEdgeAllowed(tc.relType, tc.from, tc.to, nil)
			if got != tc.expected {
				t.Errorf("IsEdgeAllowed(%q, %q, %q, nil) = %v; want %v",
					tc.relType, tc.from, tc.to, got, tc.expected)
			}
		})
	}
}

func TestChangeEntityEdgeConstraints(t *testing.T) {
	tests := []struct {
		name     string
		relType  RelationType
		from     EntityType
		to       EntityType
		expected bool
	}{
		// CHG can be From in covers
		{"covers/change→requirement", RelationCovers, EntityTypeChange, EntityTypeRequirement, true},
		{"covers/change→decision", RelationCovers, EntityTypeChange, EntityTypeDecision, true},
		{"covers/change→interface", RelationCovers, EntityTypeChange, EntityTypeInterface, true},

		// CHG cannot be From in belongs_to, precedes, blocks
		{"belongs_to/change→plan INVALID", RelationBelongsTo, EntityTypeChange, EntityTypePlan, false},
		{"precedes/change→phase INVALID", RelationPrecedes, EntityTypeChange, EntityTypePhase, false},
		{"blocks/change→phase INVALID", RelationBlocks, EntityTypeChange, EntityTypePhase, false},

		// CHG↔CHG not allowed for any relation type (except special cases like supersedes same-type)
		{"covers/change→change INVALID", RelationCovers, EntityTypeChange, EntityTypeChange, false},
		{"belongs_to/change→change INVALID", RelationBelongsTo, EntityTypeChange, EntityTypeChange, false},
		{"precedes/change→change INVALID", RelationPrecedes, EntityTypeChange, EntityTypeChange, false},
		{"blocks/change→change INVALID", RelationBlocks, EntityTypeChange, EntityTypeChange, false},
		{"delivers/change→change INVALID", RelationDelivers, EntityTypeChange, EntityTypeChange, false},

		// CHG not in delivers From
		{"delivers/change→requirement INVALID", RelationDelivers, EntityTypeChange, EntityTypeRequirement, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsEdgeAllowed(tc.relType, tc.from, tc.to, nil)
			if got != tc.expected {
				t.Errorf("IsEdgeAllowed(%q, %q, %q, nil) = %v; want %v",
					tc.relType, tc.from, tc.to, got, tc.expected)
			}
		})
	}
}
