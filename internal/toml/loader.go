package spectoml

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/tyeongkim/spec-graph/internal/model"
)

// symmetricRelationTypes lists relation types that must be stored in the
// lexicographically smaller ID's file. Only truly symmetric (undirected)
// relations belong here; directional relations like supersedes must NOT
// be included because their from→to direction carries semantic meaning.
var symmetricRelationTypes = map[model.RelationType]bool{
	model.RelationConflictsWith: true,
}

// Store manages TOML file I/O for spec-graph entities and history.
type Store struct {
	root string // .spec-graph/ directory path
}

// NewStore creates a Store rooted at the given .spec-graph/ directory path.
func NewStore(root string) *Store {
	return &Store{root: root}
}

// EntityPath returns the filesystem path for an entity file.
func (s *Store) EntityPath(id string, entityType model.EntityType) string {
	return filepath.Join(s.root, "entities", string(entityType), id+".toml")
}

// HistoryPath returns the filesystem path for a history file.
func (s *Store) HistoryPath(entityID string) string {
	return filepath.Join(s.root, "history", entityID+".toml")
}

// Init creates the directory structure: entities/{type}/ for all entity types + history/.
func (s *Store) Init() error {
	for et := range model.TypePrefixMap {
		dir := filepath.Join(s.root, "entities", string(et))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("init entity dir %q: %w", dir, err)
		}
	}

	histDir := filepath.Join(s.root, "history")
	if err := os.MkdirAll(histDir, 0o755); err != nil {
		return fmt.Errorf("init history dir: %w", err)
	}

	return nil
}

// ReadEntity reads and parses an entity TOML file, validating that the content
// matches the expected path (ID matches filename, type matches directory).
func (s *Store) ReadEntity(id string, entityType model.EntityType) (*EntityFile, error) {
	path := s.EntityPath(id, entityType)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read entity %q: %w", id, err)
	}

	var ef EntityFile
	if err := toml.Unmarshal(data, &ef); err != nil {
		return nil, fmt.Errorf("parse entity %q (%s): %w", id, path, err)
	}

	if ef.ID != id {
		return nil, fmt.Errorf("entity file %s: content ID %q does not match filename %q", path, ef.ID, id)
	}
	if ef.Type != entityType {
		return nil, fmt.Errorf("entity file %s: content type %q does not match directory %q", path, ef.Type, entityType)
	}

	return &ef, nil
}

// WriteEntity writes an entity file using the canonical writer with atomic write semantics.
// It enforces the symmetric relation rule before writing.
func (s *Store) WriteEntity(ef *EntityFile) error {
	if err := s.enforceSymmetricRelations(ef); err != nil {
		return err
	}

	path := s.EntityPath(ef.ID, ef.Type)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create entity dir for %q: %w", ef.ID, err)
	}

	content := MarshalEntityFile(*ef)
	return atomicWrite(path, []byte(content))
}

// DeleteEntity removes an entity file from disk.
func (s *Store) DeleteEntity(id string, entityType model.EntityType) error {
	path := s.EntityPath(id, entityType)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete entity %q: %w", id, err)
	}
	return nil
}

// EntityExists reports whether the entity file exists on disk.
func (s *Store) EntityExists(id string, entityType model.EntityType) bool {
	path := s.EntityPath(id, entityType)
	_, err := os.Stat(path)
	return err == nil
}

// ListEntities walks all entity directories and returns parsed EntityFiles.
func (s *Store) ListEntities() ([]EntityFile, error) {
	entitiesDir := filepath.Join(s.root, "entities")

	var results []EntityFile

	err := filepath.WalkDir(entitiesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".toml") {
			return nil
		}

		parentDir := filepath.Base(filepath.Dir(path))
		entityType := model.EntityType(parentDir)
		id := strings.TrimSuffix(d.Name(), ".toml")

		ef, err := s.ReadEntity(id, entityType)
		if err != nil {
			return fmt.Errorf("list entities: %w", err)
		}

		results = append(results, *ef)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return results, nil
}

// ReadHistory reads and parses a history TOML file for the given entity ID.
func (s *Store) ReadHistory(entityID string) (*HistoryFile, error) {
	path := s.HistoryPath(entityID)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read history %q: %w", entityID, err)
	}

	var hf HistoryFile
	if err := toml.Unmarshal(data, &hf); err != nil {
		return nil, fmt.Errorf("parse history %q (%s): %w", entityID, path, err)
	}

	return &hf, nil
}

// AppendHistory reads the existing history file (or creates a new one), appends
// the entry, and writes back atomically.
func (s *Store) AppendHistory(entityID string, entry HistoryEntry) error {
	path := s.HistoryPath(entityID)

	var hf HistoryFile
	data, err := os.ReadFile(path)
	if err == nil {
		if err := toml.Unmarshal(data, &hf); err != nil {
			return fmt.Errorf("parse history %q for append: %w", entityID, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read history %q for append: %w", entityID, err)
	} else {
		hf.EntityID = entityID
	}

	hf.Entries = append(hf.Entries, entry)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create history dir for %q: %w", entityID, err)
	}

	content := MarshalHistoryFile(hf)
	return atomicWrite(path, []byte(content))
}

// DeleteHistory removes a history file from disk.
func (s *Store) DeleteHistory(entityID string) error {
	path := s.HistoryPath(entityID)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete history %q: %w", entityID, err)
	}
	return nil
}

// enforceSymmetricRelations validates that symmetric relations (conflicts_with,
// supersedes) are stored in the lexicographically smaller ID's file.
func (s *Store) enforceSymmetricRelations(ef *EntityFile) error {
	for _, rel := range ef.Relations {
		if !symmetricRelationTypes[rel.Type] {
			continue
		}
		if ef.ID > rel.To {
			return fmt.Errorf(
				"symmetric relation %q from %q to %q must be stored in the lexicographically smaller ID's file (%q); move it to %q's file",
				rel.Type, ef.ID, rel.To, rel.To, rel.To,
			)
		}
	}
	return nil
}

// atomicWrite writes data to a temporary file in the same directory as path,
// then renames it to path. This prevents half-written files.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	tmp, err := os.CreateTemp(dir, "."+base+".tmp.*")
	if err != nil {
		return fmt.Errorf("create temp file for %q: %w", path, err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file for %q: %w", path, err)
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file for %q: %w", path, err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		// On Windows, os.Rename fails if destination exists. Remove and retry.
		if removeErr := os.Remove(path); removeErr == nil {
			if retryErr := os.Rename(tmpName, path); retryErr == nil {
				return nil
			}
		}
		os.Remove(tmpName)
		return fmt.Errorf("rename temp file to %q: %w", path, err)
	}

	return nil
}
