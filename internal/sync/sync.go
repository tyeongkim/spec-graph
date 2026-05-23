// Package sync coordinates the TOML source of truth with the SQLite query index.
// It detects changes via content hashing and triggers atomic rebuilds.
package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tyeongkim/spec-graph/internal/index"
	"github.com/tyeongkim/spec-graph/internal/model"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
)

const metaKeyFingerprint = "toml_fingerprint"

// Syncer coordinates TOML source of truth with the SQLite query index.
type Syncer struct {
	store *spectoml.Store
	idx   *index.Index
	root  string
}

// NewSyncer creates a Syncer that bridges the TOML store and SQLite index.
// root is the .spec-graph/ directory path (same root used by the Store).
func NewSyncer(store *spectoml.Store, idx *index.Index, root string) *Syncer {
	return &Syncer{
		store: store,
		idx:   idx,
		root:  root,
	}
}

// EnsureFresh checks if the index is stale and rebuilds if needed.
// Returns true if a rebuild was performed.
func (s *Syncer) EnsureFresh() (bool, error) {
	fingerprint, err := s.ComputeFingerprint()
	if err != nil {
		return false, fmt.Errorf("compute fingerprint: %w", err)
	}

	stored, err := s.idx.GetMeta(metaKeyFingerprint)
	if err != nil {
		return false, fmt.Errorf("get stored fingerprint: %w", err)
	}

	if fingerprint == stored {
		return false, nil
	}

	if err := s.ForceRebuild(); err != nil {
		return false, err
	}
	return true, nil
}

// ForceRebuild unconditionally rebuilds the index from TOML files.
func (s *Syncer) ForceRebuild() error {
	entityFiles, parseErrors := s.loadAllEntities()

	entityRecords := make([]index.EntityRecord, 0, len(entityFiles))
	var relationRecords []index.RelationRecord

	for _, ef := range entityFiles {
		entity, err := ef.ToEntity()
		if err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("convert entity %q: %w", ef.ID, err))
			continue
		}

		metaStr := "{}"
		if len(entity.Metadata) > 0 {
			metaStr = string(entity.Metadata)
		}

		entityRecords = append(entityRecords, index.EntityRecord{
			ID:          entity.ID,
			Type:        string(entity.Type),
			Layer:       string(entity.Layer),
			Status:      string(entity.Status),
			Title:       entity.Title,
			Description: entity.Description,
			Metadata:    metaStr,
			FilePath:    s.store.EntityPath(ef.ID, ef.Type),
			CreatedAt:   entity.CreatedAt,
			UpdatedAt:   entity.UpdatedAt,
		})

		relations, err := ef.ToRelations()
		if err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("convert relations for %q: %w", ef.ID, err))
			continue
		}

		for _, r := range relations {
			relMeta := "{}"
			if len(r.Metadata) > 0 {
				relMeta = string(r.Metadata)
			}
			relationRecords = append(relationRecords, index.RelationRecord{
				FromID:   r.FromID,
				ToID:     r.ToID,
				Type:     string(r.Type),
				Layer:    string(r.Layer),
				Weight:   r.Weight,
				Metadata: relMeta,
			})
		}
	}

	if err := s.idx.Rebuild(entityRecords, relationRecords); err != nil {
		return fmt.Errorf("rebuild index: %w", err)
	}

	fingerprint, err := s.ComputeFingerprint()
	if err != nil {
		return fmt.Errorf("compute fingerprint after rebuild: %w", err)
	}

	if err := s.idx.SetMeta(metaKeyFingerprint, fingerprint); err != nil {
		return fmt.Errorf("store fingerprint: %w", err)
	}

	if len(parseErrors) > 0 {
		return &RebuildError{Errors: parseErrors}
	}
	return nil
}

// ComputeFingerprint walks TOML entity files and returns a content-based hash.
func (s *Syncer) ComputeFingerprint() (string, error) {
	entitiesDir := filepath.Join(s.root, "entities")

	if _, err := os.Stat(entitiesDir); os.IsNotExist(err) {
		h := sha256.Sum256(nil)
		return hex.EncodeToString(h[:]), nil
	}

	type fileEntry struct {
		relPath string
		hash    string
	}

	var entries []fileEntry

	err := filepath.WalkDir(entitiesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".toml") {
			return nil
		}

		relPath, err := filepath.Rel(entitiesDir, path)
		if err != nil {
			return fmt.Errorf("relative path for %q: %w", path, err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %q: %w", path, err)
		}

		h := sha256.Sum256(content)
		entries = append(entries, fileEntry{
			relPath: relPath,
			hash:    hex.EncodeToString(h[:]),
		})
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk entities: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].relPath < entries[j].relPath
	})

	var builder strings.Builder
	for i, e := range entries {
		if i > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(e.relPath)
		builder.WriteByte(':')
		builder.WriteString(e.hash)
	}

	h := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(h[:]), nil
}

// loadAllEntities walks entity files, parsing each one. Files that fail to
// parse are collected as errors but do not prevent other files from loading.
func (s *Syncer) loadAllEntities() ([]spectoml.EntityFile, []error) {
	entitiesDir := filepath.Join(s.root, "entities")

	var results []spectoml.EntityFile
	var errs []error

	if _, err := os.Stat(entitiesDir); os.IsNotExist(err) {
		return nil, nil
	}

	_ = filepath.WalkDir(entitiesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			errs = append(errs, fmt.Errorf("walk error at %q: %w", path, err))
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".toml") {
			return nil
		}

		parentDir := filepath.Base(filepath.Dir(path))
		id := strings.TrimSuffix(d.Name(), ".toml")

		ef, readErr := s.store.ReadEntity(id, entityTypeFromDir(parentDir))
		if readErr != nil {
			errs = append(errs, fmt.Errorf("parse %q: %w", path, readErr))
			return nil
		}

		results = append(results, *ef)
		return nil
	})

	return results, errs
}

func entityTypeFromDir(dir string) model.EntityType {
	return model.EntityType(dir)
}

// RebuildError is returned when some TOML files failed to parse during rebuild.
// The index is still rebuilt with the files that parsed successfully.
type RebuildError struct {
	Errors []error
}

func (e *RebuildError) Error() string {
	msgs := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		msgs[i] = err.Error()
	}
	return fmt.Sprintf("rebuild completed with %d error(s): %s", len(e.Errors), strings.Join(msgs, "; "))
}

// Unwrap returns the underlying errors for use with errors.Is/As.
func (e *RebuildError) Unwrap() []error {
	return e.Errors
}

// IsRebuildError reports whether err is or wraps a RebuildError.
func IsRebuildError(err error) bool {
	var re *RebuildError
	return errors.As(err, &re)
}
