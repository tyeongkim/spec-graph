package spectoml

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/tyeongkim/spec-graph/internal/model"
)

func TestTomlQuote_Spec(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple ascii", input: "hello", want: `"hello"`},
		{name: "with tab", input: "col1\tcol2", want: `"col1\tcol2"`},
		{name: "with newline", input: "line1\nline2", want: `"line1\nline2"`},
		{name: "with backslash", input: `path\to\file`, want: `"path\\to\\file"`},
		{name: "with quote", input: `say "hi"`, want: `"say \"hi\""`},
		{name: "korean", input: "사용자 인증", want: `"사용자 인증"`},
		{name: "emoji", input: "🚀 launch", want: `"🚀 launch"`},
		{name: "null byte", input: "a\x00b", want: `"a\u0000b"`},
		{name: "mixed unicode", input: "한글\ttab\n줄바꿈", want: `"한글\ttab\n줄바꿈"`},
		{name: "line separator U+2028", input: "before\u2028after", want: `"before` + "\u2028" + `after"`},
		{name: "backspace", input: "a\bb", want: `"a\bb"`},
		{name: "form feed", input: "a\fb", want: `"a\fb"`},
		{name: "carriage return", input: "a\rb", want: `"a\rb"`},
		{name: "DEL U+007F", input: "a\x7fb", want: `"a\u007Fb"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tomlQuote(tt.input)
			if got != tt.want {
				t.Errorf("tomlQuote(%q) = %s, want %s", tt.input, got, tt.want)
			}

			var parsed struct {
				V string `toml:"v"`
			}
			tomlDoc := "v = " + got + "\n"
			if _, err := toml.Decode(tomlDoc, &parsed); err != nil {
				t.Fatalf("BurntSushi/toml cannot parse output %s: %v", got, err)
			}
			if parsed.V != tt.input {
				t.Errorf("round-trip failed: decoded %q, want %q", parsed.V, tt.input)
			}
		})
	}
}

func TestMarshalEntityFile_UnicodeRoundTrip(t *testing.T) {
	ef := EntityFile{
		Schema:      1,
		ID:          "REQ-유니코드",
		Type:        model.EntityTypeRequirement,
		Title:       "사용자 인증 요구사항",
		Description: "탭\t포함\n줄바꿈 포함\n이모지 🎉 포함",
		Status:      model.EntityStatusActive,
		Metadata: map[string]any{
			"author":   "김태영",
			"priority": "높음",
		},
		Relations: []RelationEntry{
			{To: "DEC-결정-001", Type: model.RelationDependsOn},
		},
	}

	output := MarshalEntityFile(ef)

	var parsed EntityFile
	if _, err := toml.Decode(output, &parsed); err != nil {
		t.Fatalf("failed to parse TOML with Korean text: %v\noutput:\n%s", err, output)
	}

	if parsed.ID != ef.ID {
		t.Errorf("ID: got %q, want %q", parsed.ID, ef.ID)
	}
	if parsed.Title != ef.Title {
		t.Errorf("Title: got %q, want %q", parsed.Title, ef.Title)
	}
	if parsed.Description != ef.Description {
		t.Errorf("Description: got %q, want %q", parsed.Description, ef.Description)
	}
	if parsed.Metadata["author"] != ef.Metadata["author"] {
		t.Errorf("Metadata[author]: got %q, want %q", parsed.Metadata["author"], ef.Metadata["author"])
	}

	output2 := MarshalEntityFile(parsed)
	if output2 != output {
		t.Errorf("write-read-write not idempotent:\nfirst:\n%s\nsecond:\n%s", output, output2)
	}
}
