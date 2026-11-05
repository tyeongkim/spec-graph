package specgraph

import (
	"errors"
	"fmt"

	"github.com/tyeongkim/spec-graph/internal/model"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
)

type stagedFile struct {
	id         string
	entityType model.EntityType
	file       *spectoml.EntityFile
	deleted    bool
	preImage   []byte
}

type txn struct {
	eng    *Engine
	staged map[string]*stagedFile
	order  []string
}

func transact[T any](e *Engine, fn func(*txn) (T, error)) (T, error) {
	var zero T
	tx := &txn{eng: e, staged: make(map[string]*stagedFile)}
	result, err := fn(tx)
	if err != nil {
		return zero, err
	}
	if err := tx.commit(); err != nil {
		return zero, err
	}
	return result, nil
}

func transactErr(e *Engine, fn func(*txn) error) error {
	_, err := transact(e, func(tx *txn) (struct{}, error) {
		return struct{}{}, fn(tx)
	})
	return err
}

func (t *txn) track(id string, entityType model.EntityType) (*stagedFile, error) {
	if entry, ok := t.staged[id]; ok {
		return entry, nil
	}

	preImage, err := t.eng.store.ReadEntityBytes(id, entityType)
	if err != nil {
		return nil, newError(CodeRuntime, fmt.Sprintf("read current state of %q", id), err)
	}

	entry := &stagedFile{id: id, entityType: entityType, preImage: preImage}
	t.staged[id] = entry
	t.order = append(t.order, id)
	return entry, nil
}

func (t *txn) write(ef *spectoml.EntityFile) error {
	entry, err := t.track(ef.ID, ef.Type)
	if err != nil {
		return err
	}
	entry.file = ef.Clone()
	entry.deleted = false
	return nil
}

func (t *txn) remove(id string, entityType model.EntityType) error {
	entry, err := t.track(id, entityType)
	if err != nil {
		return err
	}
	entry.file = nil
	entry.deleted = true
	return nil
}

// read returns a private copy, so callers mutate their own value and must call
// write to stage the result.
func (t *txn) read(id string, entityType model.EntityType) (*spectoml.EntityFile, error) {
	if entry, ok := t.staged[id]; ok {
		if entry.deleted {
			return nil, newError(CodeNotFound, fmt.Sprintf("entity %q not found", id), nil)
		}
		if entry.file != nil {
			return entry.file.Clone(), nil
		}
	}
	ef, err := t.eng.store.ReadEntity(id, entityType)
	if err != nil {
		return nil, newError(CodeRuntime, fmt.Sprintf("read entity %q", id), err)
	}
	return ef, nil
}

func (t *txn) exists(id string, entityType model.EntityType) bool {
	if entry, ok := t.staged[id]; ok {
		return !entry.deleted
	}
	return t.eng.store.EntityExists(id, entityType)
}

func (t *txn) commit() error {
	if len(t.order) == 0 {
		return nil
	}

	journal := transactionJournal{Version: 1}
	for _, id := range t.order {
		entry := t.staged[id]
		journal.Entries = append(journal.Entries, journalEntry{ID: id, EntityType: entry.entityType, PreImage: entry.preImage, Absent: entry.preImage == nil})
	}
	if err := writeJournal(t.eng.root, journal); err != nil {
		return journalError(t.eng.root, err)
	}
	for _, id := range t.order {
		if err := t.apply(t.staged[id]); err != nil {
			return t.rollback(journal, err)
		}
	}
	if err := syncEntityDirectories(t.eng.root, journal.Entries); err != nil {
		return t.rollback(journal, err)
	}
	if err := removeJournal(t.eng.root); err != nil {
		return journalError(t.eng.root, err)
	}
	if _, err := t.eng.syncer.EnsureFresh(); err != nil {
		return newError(CodeRuntime, "sync index after commit", err)
	}
	return nil
}

func (t *txn) apply(entry *stagedFile) error {
	if entry.deleted {
		if entry.preImage == nil {
			return nil
		}
		if err := t.eng.store.DeleteEntity(entry.id, entry.entityType); err != nil {
			return newError(CodeRuntime, fmt.Sprintf("delete entity %q", entry.id), err)
		}
		return nil
	}
	if err := t.eng.store.WriteEntity(entry.file); err != nil {
		return newError(CodeRuntime, fmt.Sprintf("write entity %q", entry.id), err)
	}
	return nil
}

func (t *txn) rollback(journal transactionJournal, cause error) error {
	if err := restoreJournal(t.eng.root, t.eng.store, journal); err != nil {
		return journalError(t.eng.root, errors.Join(cause, err))
	}
	if _, err := t.eng.syncer.EnsureFresh(); err != nil {
		return journalError(t.eng.root, errors.Join(cause, err))
	}
	return cause
}
