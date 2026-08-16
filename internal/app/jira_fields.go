package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	jira "acli-plus/internal/domain/jira"
)

// FieldAssignment is one `--field name=value` pair from the command line.
type FieldAssignment struct {
	Name  string
	Value string
	// Raw sends Value to Jira as literal JSON instead of shaping it from the
	// field's schema. It is what --field-json sets.
	Raw bool
}

// ParseFieldAssignment splits a `name=value` flag. The name may be a field id,
// key, or display name; the value may contain further "=" characters.
func ParseFieldAssignment(raw string) (FieldAssignment, error) {
	return parseAssignment("--field", raw, false)
}

// ParseFieldJSON splits a `name=json` flag for --field-json, whose value is sent
// exactly as written. Some Jira fields cannot be derived from their schema —
// Sprint reads back as an array of objects but only accepts a bare number on
// write — so there has to be a way to say precisely what goes on the wire.
func ParseFieldJSON(raw string) (FieldAssignment, error) {
	return parseAssignment("--field-json", raw, true)
}

func parseAssignment(flag, raw string, rawJSON bool) (FieldAssignment, error) {
	name, value, found := strings.Cut(raw, "=")
	if !found || strings.TrimSpace(name) == "" {
		return FieldAssignment{}, fmt.Errorf("%s expects name=value, got %q", flag, raw)
	}
	return FieldAssignment{Name: strings.TrimSpace(name), Value: value, Raw: rawJSON}, nil
}

// BuildFields resolves each assignment's field name to its id and shapes the
// value the way that field's schema requires, so callers can type
// `--field "Story Points=5"` instead of hand-writing customfield JSON.
func (s *JiraService) BuildFields(ctx context.Context, assignments []FieldAssignment) (jira.FieldValues, error) {
	if len(assignments) == 0 {
		return nil, nil
	}
	values := jira.FieldValues{}
	for _, assignment := range assignments {
		field, err := s.ResolveField(ctx, assignment.Name)
		if err != nil {
			return nil, err
		}
		var value any
		if assignment.Raw {
			if err := json.Unmarshal([]byte(assignment.Value), &value); err != nil {
				return nil, fmt.Errorf("field %s: --field-json expects valid JSON, got %q", field.Name, assignment.Value)
			}
		} else if value, err = s.coerceFieldValue(ctx, field, assignment.Value); err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}
		values[field.ID] = value
	}
	return values, nil
}

// coerceFieldValue turns a command-line string into the JSON shape Jira expects
// for that field. A value that is already JSON is passed through untouched,
// which is the escape hatch for shapes this mapping does not cover.
func (s *JiraService) coerceFieldValue(ctx context.Context, field jira.Field, raw string) (any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.EqualFold(trimmed, "null") {
		// An empty value clears the field, which is how Jira removes a value.
		return nil, nil
	}
	if decoded, ok := decodeJSONValue(trimmed); ok {
		return decoded, nil
	}

	switch field.SchemaType {
	case "number":
		number, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return nil, fmt.Errorf("expects a number, got %q", raw)
		}
		return number, nil
	case "array":
		return s.coerceArray(ctx, field, trimmed)
	case "user":
		accountID, err := s.ResolveAccountID(ctx, trimmed)
		if err != nil {
			return nil, err
		}
		return map[string]string{"accountId": accountID}, nil
	case "option", "option-with-child":
		return map[string]string{"value": trimmed}, nil
	case "project":
		return map[string]string{"key": trimmed}, nil
	case "issuetype", "priority", "resolution", "status", "securitylevel", "component", "version":
		return map[string]string{"name": trimmed}, nil
	case "date", "datetime", "string", "any", "":
		return raw, nil
	default:
		return raw, nil
	}
}

// coerceArray splits a comma-separated value and shapes each element according
// to the array's item type.
func (s *JiraService) coerceArray(ctx context.Context, field jira.Field, raw string) (any, error) {
	parts := splitList(raw)
	switch field.ItemType {
	case "user":
		users := make([]map[string]string, 0, len(parts))
		for _, part := range parts {
			accountID, err := s.ResolveAccountID(ctx, part)
			if err != nil {
				return nil, err
			}
			users = append(users, map[string]string{"accountId": accountID})
		}
		return users, nil
	case "option":
		return namedList(parts, "value"), nil
	case "component", "version", "issuetype", "group":
		return namedList(parts, "name"), nil
	default:
		// Labels and other plain string arrays.
		return parts, nil
	}
}

// decodeJSONValue passes a value through untouched when the user wrote JSON,
// e.g. --field 'customfield_10011={"value":"High"}'.
func decodeJSONValue(value string) (any, bool) {
	if !strings.HasPrefix(value, "{") && !strings.HasPrefix(value, "[") {
		return nil, false
	}
	var decoded any
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return nil, false
	}
	return decoded, true
}

// splitList splits a comma-separated flag value, trimming blanks.
func splitList(value string) []string {
	parts := make([]string, 0, 4)
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// namedList wraps each element as {key: element}, the shape Jira uses for
// components, versions and select options.
func namedList(parts []string, key string) []map[string]string {
	out := make([]map[string]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, map[string]string{key: part})
	}
	return out
}
