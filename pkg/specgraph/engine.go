package specgraph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tyeongkim/spec-graph/internal/flock"
	"github.com/tyeongkim/spec-graph/internal/index"
	"github.com/tyeongkim/spec-graph/internal/sync"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
)

// Options configures how an Engine is opened.
type Options struct {
	// Root is the path to the .spec-graph/ directory of the project.
	Root string
}

// Engine is the single entry point for operating on a spec-graph project. It
// holds handles to the TOML store, SQLite index, and syncer, and owns the
// exclusive file lock that guards concurrent access. Callers must invoke Close
// to release the lock and flush the index.
type Engine struct {
	root   string
	store  *spectoml.Store
	idx    *index.Index
	syncer *sync.Syncer
	unlock func()
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

	unlock, err := flock.Lock(filepath.Join(root, ".lock"))
	if err != nil {
		return nil, newError(CodeConflict, fmt.Sprintf("acquire lock at %q", root), err)
	}

	store := spectoml.NewStore(root)

	idx, err := index.Open(filepath.Join(root, "graph.db"))
	if err != nil {
		unlock()
		return nil, newError(CodeRuntime, fmt.Sprintf("open index at %q", root), err)
	}

	syncer := sync.NewSyncer(store, idx, root)

	if _, err := syncer.EnsureFresh(); err != nil {
		_ = idx.Close()
		unlock()
		return nil, newError(CodeRuntime, fmt.Sprintf("sync index at %q", root), err)
	}

	return &Engine{
		root:   root,
		store:  store,
		idx:    idx,
		syncer: syncer,
		unlock: unlock,
	}, nil
}

// Close releases the resources held by the Engine: it closes the SQLite index
// and releases the exclusive file lock. It returns any error encountered while
// closing the index. Close is safe to call multiple times; subsequent calls are
// no-ops. Close does not accept a context.
func (e *Engine) Close() error {
	if e.unlock == nil {
		return nil
	}
	err := e.idx.Close()
	e.unlock()
	e.unlock = nil
	return err
}

// Root returns the path to the .spec-graph/ directory.
func (e *Engine) Root() string {
	return e.root
}

// Index returns the underlying SQLite index for low-level read operations.
// Callers must not close the index directly.
func (e *Engine) Index() *index.Index {
	return e.idx
}

// Syncer returns the underlying syncer for low-level operations.
func (e *Engine) Syncer() *sync.Syncer {
	return e.syncer
}

// Store returns the underlying TOML store for low-level operations.
func (e *Engine) Store() *spectoml.Store {
	return e.store
}
