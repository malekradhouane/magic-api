package postgres

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"

	"github.com/oklog/ulid"

	"github.com/malekradhouane/magic/utils/logtool"
)

var (
	logger           = logtool.SetupLogger("[STORE/POSTEGRES]")
	ErrDuplicatedKey = errors.New("duplicate key error")

	// ErrClientNotInitialised is returned by MustClientInitialised when the
	// package-level PostgreSQL client has not been created/configured yet.
	// Surfaced as an error (no more os.Exit) so callers can decide what to do.
	ErrClientNotInitialised = errors.New("postgres client is not initialised")
)

type StringArray []string

// MustClientInitialized verifies that the package-level postgres client has
// been created and connected. It returns ErrClientNotInitialised when any
// pre-requisite is missing instead of terminating the process — callers
// (typically NewXxxStore) must bubble the error up to main().
//
// TODO: remove the package-level singleton entirely and inject *Client into
// every NewXxxStore constructor (clean-architecture rule).
func MustClientInitialized(c *Client) error {
	if c == nil {
		logger.Error("Postgres client not created (nil)")
		return ErrClientNotInitialised
	}
	s := c.Session()
	if s == nil {
		logger.Error("Postgres client session not created (nil)")
		return ErrClientNotInitialised
	}
	if s.GetDB() == nil {
		logger.Error("Postgres DB object not created (nil)")
		return ErrClientNotInitialised
	}
	return nil
}

func generateUUID() string {
	return ulid.MustNew(ulid.Now(), nil).String()
}

// wrapPgError normalises a PostgreSQL driver error so callers can inspect it
// with errors.Is(err, ErrDuplicatedKey).
//
// TODO: replace the string match with a typed check against *pgconn.PgError
// (code "23505") once jackc/pgx is promoted to a direct dependency.
func wrapPgError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
		return fmt.Errorf("%w: %v", ErrDuplicatedKey, err)
	}
	return err
}

// isEmailDuplicateError reports whether err is a unique-violation on the users email index.
func isEmailDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "idx_users_email") ||
		(strings.Contains(msg, "duplicate key") && strings.Contains(msg, "email"))
}

func (sa *StringArray) Scan(src any) error {
	switch v := src.(type) {
	case []byte:
		*sa = strings.Split(string(v), ",")
	case string:
		*sa = strings.Split(v, ",")
	default:
		return errors.New("src value cannot be cast to []byte or string")
	}
	return nil
}

func (sa StringArray) Value() (driver.Value, error) {
	if len(sa) == 0 {
		return nil, nil
	}
	return strings.Join(sa, ","), nil
}
