package interfaces

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// SettingsValue is a JSONB blob stored as the value column of a settings row.
type SettingsValue map[string]interface{}

// Scan implements sql.Scanner for GORM to read JSONB.
func (sv *SettingsValue) Scan(src interface{}) error {
	switch v := src.(type) {
	case []byte:
		return json.Unmarshal(v, sv)
	case string:
		return json.Unmarshal([]byte(v), sv)
	default:
		return errors.New("unsupported type for SettingsValue")
	}
}

// Value implements driver.Valuer for GORM to write JSONB.
func (sv SettingsValue) Value() (driver.Value, error) {
	return json.Marshal(sv)
}

// Setting represents a single settings row keyed by a unique string.
type Setting struct {
	Key       string        `gorm:"type:varchar(100);primaryKey" json:"key"`
	Value     SettingsValue `gorm:"type:jsonb;not null;default:'{}'" json:"value"`
	CreatedAt time.Time     `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time     `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName specifies the table name for Setting
func (Setting) TableName() string {
	return "settings"
}
