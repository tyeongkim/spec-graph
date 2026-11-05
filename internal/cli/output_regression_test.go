package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

type failingOutput struct{ err error }

func (w failingOutput) Write([]byte) (int, error) { return 0, w.err }

func TestJSONOutputFailureIsReturned(t *testing.T) {
	expected := errors.New("output unavailable")
	cmd := &cobra.Command{}
	cmd.SetOut(failingOutput{expected})
	if err := writeJSON(cmd, map[string]bool{"ok": true}); !errors.Is(err, expected) {
		t.Fatalf("output failure = %v, want %v", err, expected)
	}
}

func TestValidatePreservesOperationalErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "entities"), 0755); err != nil {
		t.Fatal(err)
	}
	eng, err := specgraph.Open(context.Background(), specgraph.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	old := engine
	engine = eng
	defer func() { engine = old }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := &cobra.Command{}
	cmd.Flags().String("layer", "all", "")
	cmd.Flags().String("phase", "", "")
	cmd.Flags().String("entity", "", "")
	cmd.SetContext(ctx)
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	for _, flag := range []string{"", "phase", "entity"} {
		if flag != "" {
			if err := cmd.Flags().Set(flag, "PHS-001"); err != nil {
				t.Fatal(err)
			}
		}
		err := validateCmd.RunE(cmd, nil)
		var exit *exitError
		if !errors.As(err, &exit) || exit.code != 1 || !errors.Is(err, context.Canceled) {
			t.Fatalf("%s cancellation rewritten: %v; stderr=%s", flag, err, stderr.String())
		}
		if flag != "" {
			if err := cmd.Flags().Set(flag, ""); err != nil {
				t.Fatal(err)
			}
		}
	}
}
