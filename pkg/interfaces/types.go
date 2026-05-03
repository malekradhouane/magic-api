package interfaces

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

// StringArray is a custom type that maps to PostgreSQL's text[].
// Implements sql.Scanner and driver.Valuer so GORM can persist it natively.
type StringArray []string

// Scan implements sql.Scanner
func (a *StringArray) Scan(src interface{}) error {
	if src == nil {
		*a = nil
		return nil
	}

	var raw string
	switch v := src.(type) {
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("unsupported type %T for StringArray", src)
	}

	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		*a = []string{}
		return nil
	}
	if !strings.HasPrefix(raw, "{") || !strings.HasSuffix(raw, "}") {
		return fmt.Errorf("invalid array format: %q", raw)
	}

	inner := strings.TrimSuffix(strings.TrimPrefix(raw, "{"), "}")
	if inner == "" {
		*a = []string{}
		return nil
	}

	parts := strings.Split(inner, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"`)
		p = strings.ReplaceAll(p, `\"`, `"`)
		out = append(out, p)
	}
	*a = out
	return nil
}

// Value implements driver.Valuer
func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return "{}", nil
	}
	escaped := make([]string, len(a))
	for i, s := range a {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		escaped[i] = `"` + s + `"`
	}
	return "{" + strings.Join(escaped, ",") + "}", nil
}

// GormDataType is used by GORM for migrations
func (StringArray) GormDataType() string { return "text[]" }
