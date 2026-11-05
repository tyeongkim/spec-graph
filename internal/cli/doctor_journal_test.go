package cli_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDoctorReportsLeftoverJournal(t *testing.T) {
	dbFile := initTestProject(t)
	path := filepath.Join(filepath.Dir(dbFile), "txn.journal")
	if err := os.WriteFile(path, []byte(`{"version":1,"entries":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	r, report, check := runDoctorCheck(t, t.TempDir(), dbFile, "transaction_journal")
	if r.exitCode != 2 || report.Healthy || check.Status != "fail" || len(check.Issues) != 1 || check.Issues[0].File != "txn.journal" {
		t.Fatalf("exit = %d, report = %+v, check = %+v", r.exitCode, report, check)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("doctor changed journal: %v", err)
	}
}
