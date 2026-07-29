package validate

import "github.com/tyeongkim/spec-graph/internal/model"

type validationSubjects struct {
	scoped   bool
	execIDs  map[string]bool
	phaseIDs map[string]bool
	taskIDs  map[string]bool
}

func resolveValidationSubjects(opts ValidateOptions, rf RelationFetcher, ef EntityFetcher) (validationSubjects, error) {
	if opts.Phase == nil && opts.Task == nil {
		return validationSubjects{}, nil
	}

	subjects := validationSubjects{
		scoped:   true,
		execIDs:  make(map[string]bool),
		phaseIDs: make(map[string]bool),
		taskIDs:  make(map[string]bool),
	}
	if opts.Phase != nil {
		subjects.execIDs[*opts.Phase] = true
		subjects.phaseIDs[*opts.Phase] = true
	}
	if opts.Task != nil {
		subjects.execIDs[*opts.Task] = true
		subjects.taskIDs[*opts.Task] = true
	}

	taskType := model.EntityTypeTask
	execLayer := model.LayerExec
	tasks, err := ef.List(EntityListFilters{Type: &taskType, Layer: &execLayer})
	if err != nil {
		return validationSubjects{}, err
	}
	for _, task := range tasks {
		relations, err := rf.GetByEntity(task.ID)
		if err != nil {
			return validationSubjects{}, err
		}
		for _, relation := range relations {
			if relation.FromID != task.ID || relation.Type != model.RelationBelongsTo {
				continue
			}
			if opts.Phase != nil && relation.ToID == *opts.Phase {
				subjects.execIDs[task.ID] = true
				subjects.taskIDs[task.ID] = true
			}
			if opts.Task != nil && task.ID == *opts.Task {
				subjects.phaseIDs[relation.ToID] = true
			}
		}
	}

	return subjects, nil
}
