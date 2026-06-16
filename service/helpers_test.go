package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// parseUUIDOpt
// ---------------------------------------------------------------------------

func TestParseUUIDOpt_Empty(t *testing.T) {
	t.Parallel()

	got, err := parseUUIDOpt("")
	assert.NoError(t, err)
	assert.Nil(t, got)
}

func TestParseUUIDOpt_Valid(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	got, err := parseUUIDOpt(id.String())
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, id, *got)
}

func TestParseUUIDOpt_Invalid(t *testing.T) {
	t.Parallel()

	_, err := parseUUIDOpt("not-a-uuid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid UUID")
}
