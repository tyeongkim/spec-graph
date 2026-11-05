package specgraph

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/tyeongkim/spec-graph/internal/model"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
)

const journalFilename = "txn.journal"

type transactionJournal struct {
	Version int            `json:"version"`
	Entries []journalEntry `json:"entries"`
}

type journalEntry struct {
	ID         string           `json:"id"`
	EntityType model.EntityType `json:"entity_type"`
	PreImage   []byte           `json:"pre_image"`
	Absent     bool             `json:"absent"`
}

func journalError(root string, err error) error {
	return newError(CodeRuntime, fmt.Sprintf("transaction journal %q: %v; run spec-graph doctor", filepath.Join(root, journalFilename), err), err)
}

func writeJournal(root string, journal transactionJournal) error {
	data, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(root, ".txn.journal.tmp.*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), filepath.Join(root, journalFilename)); err != nil {
		return err
	}
	return syncDirectory(root)
}

func removeJournal(root string) error {
	if err := os.Remove(filepath.Join(root, journalFilename)); err != nil {
		return err
	}
	return syncDirectory(root)
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		if unsupportedDirectorySync(err) {
			return nil
		}
		return err
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil && !unsupportedDirectorySync(err) {
		return err
	}
	return nil
}

func unsupportedDirectorySync(err error) bool {
	const windowsInvalidHandle = syscall.Errno(6)
	return runtime.GOOS == "windows" && (errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.EBADF) || errors.Is(err, windowsInvalidHandle))
}

func syncEntityDirectories(root string, entries []journalEntry) error {
	directories := map[string]bool{filepath.Join(root, "entities"): true}
	for _, entry := range entries {
		directories[filepath.Join(root, "entities", string(entry.EntityType))] = true
	}
	for path := range directories {
		if err := syncDirectory(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func restoreJournal(root string, store *spectoml.Store, journal transactionJournal) error {
	var restoreErr error
	for _, entry := range journal.Entries {
		current, err := store.ReadEntityBytes(entry.ID, entry.EntityType)
		if err == nil && !(bytes.Equal(current, entry.PreImage) && (current == nil) == entry.Absent) {
			if entry.Absent {
				err = store.DeleteEntity(entry.ID, entry.EntityType)
			} else {
				err = store.WriteEntityBytes(entry.ID, entry.EntityType, entry.PreImage)
			}
		}
		if err != nil {
			restoreErr = errors.Join(restoreErr, err)
		}
	}
	if restoreErr != nil {
		return restoreErr
	}
	if err := syncEntityDirectories(root, journal.Entries); err != nil {
		return err
	}
	return removeJournal(root)
}

func recoverJournal(root string, store *spectoml.Store) (bool, error) {
	data, err := os.ReadFile(filepath.Join(root, journalFilename))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, journalError(root, err)
	}
	var journal transactionJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		return false, journalError(root, err)
	}
	if journal.Version != 1 || len(journal.Entries) == 0 {
		return false, journalError(root, fmt.Errorf("invalid journal version or empty entries"))
	}
	for _, entry := range journal.Entries {
		if entry.Absent && entry.PreImage != nil || !entry.Absent && entry.PreImage == nil {
			return false, journalError(root, fmt.Errorf("invalid pre-image for %q", entry.ID))
		}
		if _, err := store.ReadEntityBytes(entry.ID, entry.EntityType); err != nil {
			return false, journalError(root, err)
		}
	}
	if err := restoreJournal(root, store, journal); err != nil {
		return false, journalError(root, err)
	}
	return true, nil
}
