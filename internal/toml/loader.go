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

// IsSymmetricRelation reports whether a relation is undirected, and so must be
// stored in the lexicographically smaller ID's file.
func IsSymmetricRelation(t model.RelationType) bool {
	return symmetricRelationTypes[t]
}

// Store manages TOML file I/O for spec-graph entities.
type Store struct {
	root string // .spec-graph/ directory path
}

// NewStore creates a Store rooted at the given .spec-graph/ directory path.
func NewStore(root string) *Store {
	return &Store{root: root}
}

// EntityPath returns the filesystem path for an entity file without validating
// id or entityType. Callers deriving either value from request data must use
// safeEntityPath.
func (s *Store) EntityPath(id string, entityType model.EntityType) string {
	return filepath.Join(s.root, "entities", string(entityType), id+".toml")
}

// validatePathComponent rejects values unusable as a single path component:
// empty, ".", "..", or containing a separator.
func validatePathComponent(kind, value string) error {
	if value == "" {
		return fmt.Errorf("entity %s must not be empty", kind)
	}
	if value == "." || value == ".." {
		return fmt.Errorf("entity %s %q is not a valid path component", kind, value)
	}
	if strings.ContainsRune(value, '/') || strings.ContainsRune(value, os.PathSeparator) {
		return fmt.Errorf("entity %s %q must not contain a path separator", kind, value)
	}
	if filepath.Base(value) != value {
		return fmt.Errorf("entity %s %q is not a valid path component", kind, value)
	}
	return nil
}

// safeEntityPath resolves an entity file path, guaranteeing the result stays
// inside <root>/entities regardless of the id and entityType supplied.
func (s *Store) safeEntityPath(id string, entityType model.EntityType) (string, error) {
	if err := validatePathComponent("type", string(entityType)); err != nil {
		return "", err
	}
	if err := validatePathComponent("ID", id); err != nil {
		return "", err
	}

	entitiesDir := filepath.Join(s.root, "entities")
	path := filepath.Join(entitiesDir, string(entityType), id+".toml")

	rel, err := filepath.Rel(entitiesDir, path)
	if err != nil {
		return "", fmt.Errorf("resolve entity path for %q: %w", id, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("entity path for %q (%s) escapes the entities directory", id, entityType)
	}

	return path, nil
}

func (s *Store) Init() error {
	entitiesDir := filepath.Join(s.root, "entities")
	if err := os.MkdirAll(entitiesDir, 0o755); err != nil {
		return fmt.Errorf("init entities dir: %w", err)
	}

	return nil
}

// ReadEntity reads and parses an entity TOML file, validating that the content
// matches the expected path (ID matches filename, type matches directory).
func (s *Store) ReadEntity(id string, entityType model.EntityType) (*EntityFile, error) {
	path, err := s.safeEntityPath(id, entityType)
	if err != nil {
		return nil, err
	}

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

	path, err := s.safeEntityPath(ef.ID, ef.Type)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create entity dir for %q: %w", ef.ID, err)
	}

	content := MarshalEntityFile(*ef)
	return atomicWrite(path, []byte(content))
}

// DeleteEntity removes an entity file from disk.
func (s *Store) DeleteEntity(id string, entityType model.EntityType) error {
	path, err := s.safeEntityPath(id, entityType)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete entity %q: %w", id, err)
	}
	return nil
}

// ReadEntityBytes returns the raw contents of an entity file, or nil when the
// file does not exist.
func (s *Store) ReadEntityBytes(id string, entityType model.EntityType) ([]byte, error) {
	path, err := s.safeEntityPath(id, entityType)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read entity bytes %q: %w", id, err)
	}
	return data, nil
}

// WriteEntityBytes replaces an entity file with exact bytes, bypassing
// canonical serialization so contents captured earlier restore byte-identically.
func (s *Store) WriteEntityBytes(id string, entityType model.EntityType, data []byte) error {
	path, err := s.safeEntityPath(id, entityType)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create entity dir for %q: %w", id, err)
	}
	return atomicWrite(path, data)
}

// EntityExists reports whether the entity file exists on disk. An id or type
// that cannot form a safe path is reported as non-existent.
func (s *Store) EntityExists(id string, entityType model.EntityType) bool {
	path, err := s.safeEntityPath(id, entityType)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
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

// enforceSymmetricRelations validates that symmetric relations are stored in
// the lexicographically smaller ID's file. See symmetricRelationTypes for which
// relations that covers.
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
// then renames it to path. The temp file is fsynced before the rename so a
// crash cannot leave the rename durable while the contents are not.
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

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("sync temp file for %q: %w", path, err)
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
