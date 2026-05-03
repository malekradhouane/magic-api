package service

import (
	"fmt"

	"github.com/google/uuid"
)

// parseUUIDOpt parses a non-empty UUID string and returns a pointer (or nil if empty)
func parseUUIDOpt(s string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("invalid UUID %q: %w", s, err)
	}
	return &u, nil
}
