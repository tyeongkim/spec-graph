package cli

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
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
