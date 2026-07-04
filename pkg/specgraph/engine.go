package specgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	stdsync "sync"
	"time"

	"github.com/tyeongkim/spec-graph/internal/index"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/sync"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
)

// Options configures how an Engine is opened.
type Options struct {
	// Root is the path to the .spec-graph/ directory of the project.
	Root string
	// LockTimeout bounds how long each operation waits for the cross-process
	// file lock before returning a conflict error. Zero uses a default.
	LockTimeout time.Duration
}

// Engine is the single entry point for operating on a spec-graph project. It
// holds handles to the TOML store, SQLite index, and syncer, and owns the
// exclusive file lock that guards concurrent access. Callers must invoke Close
// to release the lock and flush the index.
//
// All exported operation methods are safe for concurrent use by multiple
// goroutines: read operations acquire mu for reading, while write operations
// (including Close) acquire mu for writing. Write operations hold the lock
// across the entire TOML-write + index-sync sequence so a mutation and its
// index refresh are observed atomically by concurrent readers.
type Engine struct {
	root   string
	store  *spectoml.Store
	idx    *index.Index
	syncer *sync.Syncer
	lock   *lockManager
	closed bool
	// mu guards all operations against concurrent access. Exported methods
	// acquire it (RLock for reads, Lock for writes); unexported helpers run
	// under an already-held lock and must not acquire it themselves.
	mu stdsync.RWMutex
}

// Open initializes an Engine rooted at opts.Root. It validates that the
// directory is an initialized spec-graph project, acquires an exclusive file
// lock, opens the SQLite index, and synchronizes the index with the TOML
// source of truth. The provided context is accepted for forward compatibility;
// subsystems do not yet observe cancellation.
//
// On any failure partway through, Open releases anything it has already
// acquired before returning. Errors are returned as *Error values.
func Open(ctx context.Context, opts Options) (*Engine, error) {
	_ = ctx

	root := opts.Root

	entitiesDir := filepath.Join(root, "entities")
	info, err := os.Stat(entitiesDir)
	if err != nil || !info.IsDir() {
		return nil, newError(
			CodeNotFound,
			fmt.Sprintf("spec-graph project not initialized at %q (missing entities/ directory)", root),
			err,
		)
	}

	store := spectoml.NewStore(root)

	idx, err := index.Open(filepath.Join(root, "graph.db"))
	if err != nil {
		return nil, newError(CodeRuntime, fmt.Sprintf("open index at %q", root), err)
	}

	syncer := sync.NewSyncer(store, idx, root)

	lockPath := filepath.Join(root, ".lock")
	if err := withInitialLock(lockPath, opts.LockTimeout, func() error {
		_, syncErr := syncer.EnsureFresh()
		return syncErr
	}); err != nil {
		_ = idx.Close()
		if isLockTimeout(err) {
			return nil, newError(CodeConflict, fmt.Sprintf("another spec-graph process holds the lock at %q", root), err)
		}
		return nil, newError(CodeRuntime, fmt.Sprintf("sync index at %q", root), err)
	}

	eng := &Engine{
		root:   root,
		store:  store,
		idx:    idx,
		syncer: syncer,
	}
	eng.lock = newLockManager(lockPath, opts.LockTimeout, eng.onLockAcquired)
	return eng, nil
}

// onLockAcquired runs at the 0->1 lock transition. With the file lock freshly
// held and no other in-process operation active, it reconciles the index with
// any changes another process made while this process held no lock: it reopens
// the database if the file was replaced, then rebuilds if the TOML fingerprint
// changed.
func (e *Engine) onLockAcquired() error {
	if _, err := e.idx.RefreshIfReplaced(); err != nil {
		return newError(CodeRuntime, "refresh index handle", err)
	}
	if _, err := e.syncer.EnsureFresh(); err != nil {
		return newError(CodeRuntime, "sync index", err)
	}
	return nil
}

// Close releases the resources held by the Engine by closing the SQLite index.
// The cross-process file lock is no longer held across the Engine lifetime; it
// is acquired and released per operation, so Close only needs to close the
// index. Close is safe to call multiple times; subsequent calls are no-ops.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}
	e.closed = true
	return e.idx.Close()
}

// writeLocked runs fn as a write operation. It takes the cross-process file
// lock (refreshing the index if another process changed it), then the
// in-process write lock, guaranteeing both cross-process and in-process
// exclusivity for the mutation before invoking fn.
func writeLocked[T any](e *Engine, fn func() (T, error)) (T, error) {
	var zero T
	if err := e.lock.acquire(); err != nil {
		return zero, conflictFromLock(e.root, err)
	}
	defer e.lock.release()

	e.mu.Lock()
	defer e.mu.Unlock()
	return fn()
}

// readLocked runs fn as a read operation. It takes the cross-process file lock
// (refreshing the index if another process changed it), then the in-process
// read lock, allowing concurrent in-process readers while excluding other
// processes, before invoking fn.
func readLocked[T any](e *Engine, fn func() (T, error)) (T, error) {
	var zero T
	if err := e.lock.acquire(); err != nil {
		return zero, conflictFromLock(e.root, err)
	}
	defer e.lock.release()

	e.mu.RLock()
	defer e.mu.RUnlock()
	return fn()
}

// Root returns the path to the .spec-graph/ directory.
func (e *Engine) Root() string {
	return e.root
}

// Fingerprint returns the content-based fingerprint of the TOML source files.
// It is a locked read operation used by diagnostics to detect index staleness.
func (e *Engine) Fingerprint() (string, error) {
	return readLocked(e, func() (string, error) {
		v, ferr := e.syncer.ComputeFingerprint()
		if ferr != nil {
			return "", newError(CodeRuntime, "compute fingerprint", ferr)
		}
		return v, nil
	})
}

// IndexMeta returns a metadata value stored in the index. It is a locked read
// operation used by diagnostics.
func (e *Engine) IndexMeta(key string) (string, error) {
	return readLocked(e, func() (string, error) {
		v, merr := e.idx.GetMeta(key)
		if merr != nil {
			return "", newError(CodeRuntime, fmt.Sprintf("get index meta %q", key), merr)
		}
		return v, nil
	})
}

// RelationsByEntity returns all relations referencing the given entity. It is a
// locked read operation exposed for callers that need relation adjacency
// without going through a higher-level query.
func (e *Engine) RelationsByEntity(entityID string) ([]model.Relation, error) {
	return readLocked(e, func() ([]model.Relation, error) {
		recs, rerr := e.idx.GetRelationsByEntity(entityID)
		if rerr != nil {
			return nil, newError(CodeRuntime, fmt.Sprintf("relations by entity %q", entityID), rerr)
		}
		rels := make([]model.Relation, len(recs))
		for i := range recs {
			rec := &recs[i]
			rels[i] = model.Relation{
				FromID:   rec.FromID,
				ToID:     rec.ToID,
				Type:     model.RelationType(rec.Type),
				Layer:    model.Layer(rec.Layer),
				Weight:   rec.Weight,
				Metadata: json.RawMessage(rec.Metadata),
			}
		}
		return rels, nil
	})
}

// RawQueryResult holds the column names and row maps of a raw SQL query.
type RawQueryResult struct {
	Columns []string
	Rows    []map[string]any
}

// RawQuery runs a read-only SQL query against the index under the read lock and
// returns the column names and rows as maps keyed by column name. Byte-slice
// column values are normalized to strings for stable JSON encoding. The caller
// is responsible for ensuring the query is a read-only SELECT.
func (e *Engine) RawQuery(ctx context.Context, query string) (*RawQueryResult, error) {
	return readLocked(e, func() (*RawQueryResult, error) {
		rows, qerr := e.idx.DB().QueryContext(ctx, query)
		if qerr != nil {
			return nil, newError(CodeRuntime, "query execution", qerr)
		}
		defer rows.Close()

		columns, cerr := rows.Columns()
		if cerr != nil {
			return nil, newError(CodeRuntime, "read columns", cerr)
		}

		result := &RawQueryResult{Columns: columns, Rows: []map[string]any{}}
		for rows.Next() {
			vals := make([]any, len(columns))
			ptrs := make([]any, len(columns))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if serr := rows.Scan(ptrs...); serr != nil {
				return nil, newError(CodeRuntime, "scan row", serr)
			}
			row := make(map[string]any, len(columns))
			for i, col := range columns {
				if b, ok := vals[i].([]byte); ok {
					row[col] = string(b)
				} else {
					row[col] = vals[i]
				}
			}
			result.Rows = append(result.Rows, row)
		}
		if rerr := rows.Err(); rerr != nil {
			return nil, newError(CodeRuntime, "iterate rows", rerr)
		}
		return result, nil
	})
}
