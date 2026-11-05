package index

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRawQueryWithRelativeIndexPath(t *testing.T) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	path, err := filepath.Rel(workingDirectory, filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	idx, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	rows, err := idx.QueryRaw(context.Background(), "SELECT count(*) FROM entities")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatalf("missing count row: %v", rows.Err())
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("count = %d; want 0", count)
	}
}
