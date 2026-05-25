package interfaces

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONMap stores arbitrary JSON objects in PostgreSQL JSONB columns.
type JSONMap map[string]interface{}

// Value implements driver.Valuer for JSONB persistence.
func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

// Scan implements sql.Scanner for JSONB persistence.
func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*m = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("JSONMap: unsupported type %T", value)
	}
	if len(bytes) == 0 {
		*m = JSONMap{}
		return nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(bytes, &out); err != nil {
		return err
	}
	*m = JSONMap(out)
	return nil
}

// Bool returns a boolean flag stored in the map, or false if missing.
func (m JSONMap) Bool(key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

// Clone returns a shallow copy of the map.
func (m JSONMap) Clone() JSONMap {
	if m == nil {
		return JSONMap{}
	}
	c := make(JSONMap, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}
