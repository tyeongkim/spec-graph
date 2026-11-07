package model

import "testing"

func TestValidateExecTransitionMatrix(t *testing.T) {
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

	for _, entityType := range []EntityType{EntityTypePlan, EntityTypePhase, EntityTypeTask, EntityTypeChange} {
		for _, transition := range transitions {
			for _, to := range statuses {
				t.Run(string(entityType)+" "+string(transition.from)+" to "+string(to), func(t *testing.T) {
					_, wantAllowed := transition.allowed[to]
					if gotAllowed := ValidateExecTransition(entityType, transition.from, to) == nil; gotAllowed != wantAllowed {
						t.Errorf("transition allowed = %t; want %t", gotAllowed, wantAllowed)
					}
				})
			}
		}
	}
}
