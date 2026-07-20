package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// TaskContract is the closed metadata contract for task entities.
type TaskContract struct {
	Order        int      `json:"order"`
	Instructions []string `json:"instructions"`
	Acceptance   []string `json:"acceptance"`
	MustNot      []string `json:"must_not"`
	References   []string `json:"references"`
	QA           []QAItem `json:"qa"`
}

// QAItem describes one command-based task verification and its evidence.
type QAItem struct {
	Command  string `json:"command"`
	Expected string `json:"expected"`
	Evidence string `json:"evidence"`
}

type taskContractInput struct {
	Order        int           `json:"order"`
	Instructions []string      `json:"instructions"`
	Acceptance   []string      `json:"acceptance"`
	MustNot      []string      `json:"must_not"`
	References   []string      `json:"references"`
	QA           []qaItemInput `json:"qa"`
}

type qaItemInput struct {
	Command  string  `json:"command"`
	Expected string  `json:"expected"`
	Evidence *string `json:"evidence"`
}

// DecodeTaskContract parses and validates task metadata for the given lifecycle status.
func DecodeTaskContract(metadata json.RawMessage, status EntityStatus) (TaskContract, error) {
	decoder := json.NewDecoder(bytes.NewReader(metadata))
	decoder.DisallowUnknownFields()

	var input taskContractInput
	if err := decoder.Decode(&input); err != nil {
		return TaskContract{}, fmt.Errorf("decode task metadata: %w", err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return TaskContract{}, err
	}
	if input.Order <= 0 {
		return TaskContract{}, fmt.Errorf("task metadata order must be a positive integer")
	}
	if err := validateRequiredStrings("instructions", input.Instructions, true); err != nil {
		return TaskContract{}, err
	}
	if err := validateRequiredStrings("acceptance", input.Acceptance, true); err != nil {
		return TaskContract{}, err
	}
	if err := validateRequiredStrings("must_not", input.MustNot, false); err != nil {
		return TaskContract{}, err
	}
	if err := validateRequiredStrings("references", input.References, false); err != nil {
		return TaskContract{}, err
	}
	if len(input.QA) == 0 {
		return TaskContract{}, fmt.Errorf("task metadata qa must be a non-empty array")
	}

	qa := make([]QAItem, len(input.QA))
	for i, item := range input.QA {
		if strings.TrimSpace(item.Command) == "" {
			return TaskContract{}, fmt.Errorf("task metadata qa[%d].command must be non-empty", i)
		}
		if strings.TrimSpace(item.Expected) == "" {
			return TaskContract{}, fmt.Errorf("task metadata qa[%d].expected must be non-empty", i)
		}
		if item.Evidence == nil {
			return TaskContract{}, fmt.Errorf("task metadata qa[%d].evidence is required", i)
		}
		evidencePresent := strings.TrimSpace(*item.Evidence) != ""
		if status == EntityStatusResolved && !evidencePresent {
			return TaskContract{}, fmt.Errorf("task metadata qa[%d].evidence is required when resolved", i)
		}
		if status != EntityStatusResolved && evidencePresent {
			return TaskContract{}, fmt.Errorf("task metadata qa[%d].evidence must be empty before resolution", i)
		}
		qa[i] = QAItem{Command: item.Command, Expected: item.Expected, Evidence: *item.Evidence}
	}

	return TaskContract{
		Order:        input.Order,
		Instructions: input.Instructions,
		Acceptance:   input.Acceptance,
		MustNot:      input.MustNot,
		References:   input.References,
		QA:           qa,
	}, nil
}

// ValidateTaskTransition rejects task lifecycle transitions outside the strict state machine.
func ValidateTaskTransition(from, to EntityStatus) error {
	valid := (from == EntityStatusDraft && (to == EntityStatusActive || to == EntityStatusDeprecated)) ||
		(from == EntityStatusActive && (to == EntityStatusResolved || to == EntityStatusDeprecated))
	if !valid {
		return fmt.Errorf("task status transition %q to %q is not allowed", from, to)
	}
	return nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != nil {
		if err == io.EOF {
			return nil
		}
		return fmt.Errorf("decode task metadata: %w", err)
	}
	return fmt.Errorf("decode task metadata: multiple JSON values are not allowed")
}

func validateRequiredStrings(name string, values []string, requireNonEmpty bool) error {
	if values == nil {
		return fmt.Errorf("task metadata %s is required", name)
	}
	if requireNonEmpty && len(values) == 0 {
		return fmt.Errorf("task metadata %s must be a non-empty array", name)
	}
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("task metadata %s[%d] must be non-empty", name, i)
		}
	}
	return nil
}
