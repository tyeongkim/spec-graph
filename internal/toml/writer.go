package spectoml

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// MarshalEntityFile produces canonical TOML output for an EntityFile.
// Key order: schema, id, type, title, description (if non-empty), status, created_at, updated_at, [metadata], [[relations]].
// Relations are sorted by (type ASC, to ASC).
func MarshalEntityFile(ef EntityFile) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("schema = %d\n", ef.Schema))
	b.WriteString("id = " + tomlQuote(ef.ID) + "\n")
	b.WriteString("type = " + tomlQuote(string(ef.Type)) + "\n")
	b.WriteString("title = " + tomlQuote(ef.Title) + "\n")
	if ef.Description != "" {
		b.WriteString("description = " + tomlQuote(ef.Description) + "\n")
	}
	b.WriteString("status = " + tomlQuote(string(ef.Status)) + "\n")

	if !ef.CreatedAt.IsZero() {
		b.WriteString(fmt.Sprintf("created_at = %s\n", ef.CreatedAt.Format(time.RFC3339)))
	}
	if !ef.UpdatedAt.IsZero() {
		b.WriteString(fmt.Sprintf("updated_at = %s\n", ef.UpdatedAt.Format(time.RFC3339)))
	}

	if len(ef.Metadata) > 0 {
		b.WriteByte('\n')
		b.WriteString("[metadata]\n")
		writeMapCanonical(&b, ef.Metadata)
	}

	if len(ef.Relations) > 0 {
		sorted := make([]RelationEntry, len(ef.Relations))
		copy(sorted, ef.Relations)
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].Type != sorted[j].Type {
				return sorted[i].Type < sorted[j].Type
			}
			return sorted[i].To < sorted[j].To
		})

		for _, rel := range sorted {
			b.WriteByte('\n')
			b.WriteString("[[relations]]\n")
			b.WriteString("to = " + tomlQuote(rel.To) + "\n")
			b.WriteString("type = " + tomlQuote(string(rel.Type)) + "\n")
			if rel.Weight != 0 && rel.Weight != 1.0 {
				b.WriteString(fmt.Sprintf("weight = %s\n", formatFloat(rel.Weight)))
			}
			if len(rel.Metadata) > 0 {
				b.WriteString(fmt.Sprintf("metadata = %s\n", formatInlineTable(rel.Metadata)))
			}
		}
	}

	return b.String()
}

// MarshalHistoryFile produces canonical TOML output for a HistoryFile.
// Key order per entry: action, reason, actor, detail (if non-empty), timestamp.
func MarshalHistoryFile(hf HistoryFile) string {
	var b strings.Builder

	b.WriteString("entity_id = " + tomlQuote(hf.EntityID) + "\n")

	for _, entry := range hf.Entries {
		b.WriteByte('\n')
		b.WriteString("[[entries]]\n")
		b.WriteString("action = " + tomlQuote(string(entry.Action)) + "\n")
		b.WriteString("reason = " + tomlQuote(entry.Reason) + "\n")
		b.WriteString("actor = " + tomlQuote(entry.Actor) + "\n")
		if entry.Detail != "" {
			b.WriteString("detail = " + tomlQuote(entry.Detail) + "\n")
		}
		b.WriteString(fmt.Sprintf("timestamp = %s\n", entry.Timestamp.Format("2006-01-02T15:04:05-07:00")))
	}

	return b.String()
}

// tomlQuote produces a TOML basic string (double-quoted) with proper escaping
// per the TOML specification. Unlike Go's %q verb, this only uses escape
// sequences defined by TOML: \b, \t, \n, \f, \r, \", \\, \uXXXX, \UXXXXXXXX.
func tomlQuote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')

	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch r {
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			if r == utf8.RuneError && size == 1 {
				// Invalid UTF-8 byte — escape as \uFFFD
				b.WriteString(`\uFFFD`)
			} else if unicode.IsControl(r) {
				// Control characters not handled above (U+0000–U+001F, U+007F, U+0080–U+009F)
				if r <= 0xFFFF {
					fmt.Fprintf(&b, `\u%04X`, r)
				} else {
					fmt.Fprintf(&b, `\U%08X`, r)
				}
			} else {
				// Valid printable rune — write as UTF-8
				b.WriteRune(r)
			}
		}
		i += size
	}

	b.WriteByte('"')
	return b.String()
}

func writeMapCanonical(b *strings.Builder, m map[string]any) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		b.WriteString(fmt.Sprintf("%s = %s\n", k, formatValue(m[k])))
	}
}

func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return tomlQuote(val)
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return formatFloat(val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return tomlQuote(fmt.Sprint(v))
	}
}

func formatFloat(f float64) string {
	s := fmt.Sprintf("%g", f)
	if !strings.Contains(s, ".") && !strings.Contains(s, "e") {
		s += ".0"
	}
	return s
}

func formatInlineTable(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s = %s", k, formatValue(m[k])))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
