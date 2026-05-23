package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/model"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
)

type doctorIssue struct {
	File    string `json:"file"`
	Message string `json:"message"`
}

type doctorCheck struct {
	Name   string        `json:"name"`
	Status string        `json:"status"`
	Issues []doctorIssue `json:"issues"`
}

type doctorSummary struct {
	TotalChecks int `json:"total_checks"`
	Passed      int `json:"passed"`
	Failed      int `json:"failed"`
	TotalIssues int `json:"total_issues"`
}

type doctorReport struct {
	Healthy bool          `json:"healthy"`
	Checks  []doctorCheck `json:"checks"`
	Summary doctorSummary `json:"summary"`
}

var allCheckNames = []string{
	"toml_parse",
	"id_filename_match",
	"type_directory_match",
	"duplicate_ids",
	"orphan_relations",
	"edge_matrix",
	"symmetric_relations",
	"schema_validation",
	"self_loop_relations",
	"stale_index",
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Validate TOML file integrity",
	Long:  `Validates the integrity of the TOML source of truth. Run after git merge, git pull, or when things seem broken. Reports issues but does not auto-fix.`,
	Args:  cobra.NoArgs,
	RunE:  runDoctor,
}

func init() {
	doctorCmd.Flags().String("check", "", "comma-separated list of checks to run (default: all)")
	doctorCmd.Flags().Bool("fix", false, "auto-fix issues (not yet supported)")
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	fixFlag, _ := cmd.Flags().GetBool("fix")
	if fixFlag {
		fmt.Fprintln(cmd.ErrOrStderr(), "auto-fix not yet supported")
		os.Exit(1)
		return nil
	}

	checksToRun, err := resolveChecks(cmd)
	if err != nil {
		writeError(cmd, err, 1)
		return nil
	}

	checkSet := make(map[string]bool, len(checksToRun))
	for _, c := range checksToRun {
		checkSet[c] = true
	}

	entitiesDir := filepath.Join(specRoot, "entities")
	rawFiles, walkErr := walkEntityFiles(entitiesDir)
	if walkErr != nil {
		writeError(cmd, fmt.Errorf("walk entities directory: %w", walkErr), 1)
		return nil
	}

	var checks []doctorCheck

	for _, name := range allCheckNames {
		if !checkSet[name] {
			continue
		}
		var issues []doctorIssue
		switch name {
		case "toml_parse":
			issues = checkTOMLParse(rawFiles)
		case "id_filename_match":
			issues = checkIDFilenameMatch(rawFiles)
		case "type_directory_match":
			issues = checkTypeDirectoryMatch(rawFiles)
		case "duplicate_ids":
			issues = checkDuplicateIDs(rawFiles)
		case "orphan_relations":
			issues = checkOrphanRelations(rawFiles)
		case "edge_matrix":
			issues = checkEdgeMatrix(rawFiles)
		case "symmetric_relations":
			issues = checkSymmetricRelations(rawFiles)
		case "schema_validation":
			issues = checkSchemaValidation(rawFiles)
		case "self_loop_relations":
			issues = checkSelfLoopRelations(rawFiles)
		case "stale_index":
			issues = checkStaleIndex(cmd)
		}

		status := "pass"
		if len(issues) > 0 {
			status = "fail"
		}
		checks = append(checks, doctorCheck{
			Name:   name,
			Status: status,
			Issues: issues,
		})
	}

	passed := 0
	failed := 0
	totalIssues := 0
	for _, c := range checks {
		if c.Status == "pass" {
			passed++
		} else {
			failed++
		}
		totalIssues += len(c.Issues)
	}

	report := doctorReport{
		Healthy: failed == 0,
		Checks:  checks,
		Summary: doctorSummary{
			TotalChecks: len(checks),
			Passed:      passed,
			Failed:      failed,
			TotalIssues: totalIssues,
		},
	}

	writeJSON(cmd, report)

	if failed > 0 {
		os.Exit(2)
	}
	return nil
}

func resolveChecks(cmd *cobra.Command) ([]string, error) {
	checkFlag, _ := cmd.Flags().GetString("check")
	if checkFlag == "" {
		return allCheckNames, nil
	}

	validSet := make(map[string]bool, len(allCheckNames))
	for _, n := range allCheckNames {
		validSet[n] = true
	}

	parts := strings.Split(checkFlag, ",")
	var result []string
	for _, p := range parts {
		name := strings.TrimSpace(p)
		if name == "" {
			continue
		}
		if !validSet[name] {
			return nil, fmt.Errorf("unknown check %q; valid checks: %s", name, strings.Join(allCheckNames, ", "))
		}
		result = append(result, name)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid checks specified")
	}
	return result, nil
}

type rawEntityFile struct {
	relPath  string
	absPath  string
	dirName  string
	fileName string
	parseErr error
	parsed   *spectoml.EntityFile
}

func walkEntityFiles(entitiesDir string) ([]rawEntityFile, error) {
	var files []rawEntityFile

	if _, err := os.Stat(entitiesDir); os.IsNotExist(err) {
		return files, nil
	}

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

		dirName := filepath.Base(filepath.Dir(path))
		fileName := strings.TrimSuffix(d.Name(), ".toml")

		relPath, _ := filepath.Rel(specRoot, path)

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			files = append(files, rawEntityFile{
				relPath:  relPath,
				absPath:  path,
				dirName:  dirName,
				fileName: fileName,
				parseErr: readErr,
			})
			return nil
		}

		var ef spectoml.EntityFile
		if parseErr := toml.Unmarshal(data, &ef); parseErr != nil {
			files = append(files, rawEntityFile{
				relPath:  relPath,
				absPath:  path,
				dirName:  dirName,
				fileName: fileName,
				parseErr: parseErr,
			})
			return nil
		}

		files = append(files, rawEntityFile{
			relPath:  relPath,
			absPath:  path,
			dirName:  dirName,
			fileName: fileName,
			parsed:   &ef,
		})
		return nil
	})

	return files, err
}

func checkTOMLParse(files []rawEntityFile) []doctorIssue {
	var issues []doctorIssue
	for _, f := range files {
		if f.parseErr != nil {
			issues = append(issues, doctorIssue{
				File:    f.relPath,
				Message: fmt.Sprintf("TOML parse error: %v", f.parseErr),
			})
		}
	}
	return issues
}

func checkIDFilenameMatch(files []rawEntityFile) []doctorIssue {
	var issues []doctorIssue
	for _, f := range files {
		if f.parsed == nil {
			continue
		}
		if f.parsed.ID != f.fileName {
			issues = append(issues, doctorIssue{
				File:    f.relPath,
				Message: fmt.Sprintf("ID in file is %q, expected %q", f.parsed.ID, f.fileName),
			})
		}
	}
	return issues
}

func checkTypeDirectoryMatch(files []rawEntityFile) []doctorIssue {
	var issues []doctorIssue
	for _, f := range files {
		if f.parsed == nil {
			continue
		}
		if string(f.parsed.Type) != f.dirName {
			issues = append(issues, doctorIssue{
				File:    f.relPath,
				Message: fmt.Sprintf("type in file is %q, expected %q (from directory)", string(f.parsed.Type), f.dirName),
			})
		}
	}
	return issues
}

func checkDuplicateIDs(files []rawEntityFile) []doctorIssue {
	idFiles := make(map[string][]string)
	for _, f := range files {
		if f.parsed == nil {
			continue
		}
		idFiles[f.parsed.ID] = append(idFiles[f.parsed.ID], f.relPath)
	}

	var issues []doctorIssue
	for id, paths := range idFiles {
		if len(paths) > 1 {
			for _, p := range paths {
				issues = append(issues, doctorIssue{
					File:    p,
					Message: fmt.Sprintf("duplicate entity ID %q (also in: %s)", id, strings.Join(paths, ", ")),
				})
			}
		}
	}
	return issues
}

func checkOrphanRelations(files []rawEntityFile) []doctorIssue {
	knownIDs := make(map[string]bool)
	for _, f := range files {
		if f.parsed == nil {
			continue
		}
		knownIDs[f.parsed.ID] = true
	}

	var issues []doctorIssue
	for _, f := range files {
		if f.parsed == nil {
			continue
		}
		for _, rel := range f.parsed.Relations {
			if !knownIDs[rel.To] {
				issues = append(issues, doctorIssue{
					File:    f.relPath,
					Message: fmt.Sprintf("relation to %q but entity does not exist", rel.To),
				})
			}
		}
	}
	return issues
}

func checkEdgeMatrix(files []rawEntityFile) []doctorIssue {
	idTypeMap := make(map[string]model.EntityType)
	for _, f := range files {
		if f.parsed == nil {
			continue
		}
		idTypeMap[f.parsed.ID] = f.parsed.Type
	}

	var issues []doctorIssue
	for _, f := range files {
		if f.parsed == nil {
			continue
		}
		fromType := f.parsed.Type
		for _, rel := range f.parsed.Relations {
			toType, ok := idTypeMap[rel.To]
			if !ok {
				continue
			}
			layer := model.LayerForRelationType(rel.Type)
			if !model.IsEdgeAllowed(rel.Type, fromType, toType, &layer) {
				issues = append(issues, doctorIssue{
					File: f.relPath,
					Message: fmt.Sprintf("relation %q from %s (%s) to %s (%s) not allowed by edge matrix",
						rel.Type, f.parsed.ID, fromType, rel.To, toType),
				})
			}
		}
	}
	return issues
}

func checkSymmetricRelations(files []rawEntityFile) []doctorIssue {
	var issues []doctorIssue
	for _, f := range files {
		if f.parsed == nil {
			continue
		}
		for _, rel := range f.parsed.Relations {
			if rel.Type != model.RelationConflictsWith && rel.Type != model.RelationSupersedes {
				continue
			}
			if f.parsed.ID > rel.To {
				issues = append(issues, doctorIssue{
					File: f.relPath,
					Message: fmt.Sprintf("symmetric relation %q from %q to %q must be stored in %q's file (lexicographically smaller)",
						rel.Type, f.parsed.ID, rel.To, rel.To),
				})
			}
		}
	}
	return issues
}

func checkSchemaValidation(files []rawEntityFile) []doctorIssue {
	schema := spectoml.DefaultSchema()
	var issues []doctorIssue

	for _, f := range files {
		if f.parsed == nil {
			continue
		}

		if err := schema.ValidateEntity(f.parsed.ID, string(f.parsed.Type), string(f.parsed.Status)); err != nil {
			issues = append(issues, doctorIssue{
				File:    f.relPath,
				Message: err.Error(),
			})
		}

		for _, rel := range f.parsed.Relations {
			if _, ok := schema.RelationTypes[string(rel.Type)]; !ok {
				issues = append(issues, doctorIssue{
					File:    f.relPath,
					Message: fmt.Sprintf("unknown relation type %q", rel.Type),
				})
			}
		}
	}
	return issues
}

func checkSelfLoopRelations(files []rawEntityFile) []doctorIssue {
	var issues []doctorIssue
	for _, f := range files {
		if f.parsed == nil {
			continue
		}
		for _, rel := range f.parsed.Relations {
			if f.parsed.ID == rel.To {
				issues = append(issues, doctorIssue{
					File:    f.relPath,
					Message: fmt.Sprintf("self-loop: relation %q points from %q to itself", rel.Type, f.parsed.ID),
				})
			}
		}
	}
	return issues
}

func checkStaleIndex(cmd *cobra.Command) []doctorIssue {
	var issues []doctorIssue

	currentFP, err := syncer.ComputeFingerprint()
	if err != nil {
		issues = append(issues, doctorIssue{
			File:    "",
			Message: fmt.Sprintf("failed to compute TOML fingerprint: %v", err),
		})
		return issues
	}

	storedFP, err := queryIndex.GetMeta("toml_fingerprint")
	if err != nil {
		issues = append(issues, doctorIssue{
			File:    "",
			Message: fmt.Sprintf("failed to read index fingerprint: %v", err),
		})
		return issues
	}

	if currentFP != storedFP {
		issues = append(issues, doctorIssue{
			File:    "",
			Message: fmt.Sprintf("index is stale: TOML fingerprint %q != stored %q; run sync to update", currentFP[:12]+"...", storedFP[:min(12, len(storedFP))]+"..."),
		})
	}

	return issues
}
