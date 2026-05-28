package postgres

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/oklog/ulid"

	"github.com/malekradhouane/magic/utils/logtool"
)

var (
	logger           = logtool.SetupLogger("[STORE/POSTEGRES]")
	ErrDuplicatedKey = errors.New("duplicate key error")
)

type StringArray []string

// MustClientInitialized checks if postgres is initialized
func MustClientInitialized(c *Client) {
	if c == nil {
		logger.Error("Postgres client not created (nil)")
		os.Exit(-1)
	}

	s := c.Session()

	if s == nil {
		logger.Error("Postgres client Magic session not created (nil)")
		os.Exit(-1)
	}

	if s.GetDB() == nil {
		logger.Error("Postgres DB object not created (nil)")
		os.Exit(-1)
	}

}

func generateUUID() string {
	return ulid.MustNew(ulid.Now(), nil).String()
}

func wrapPgError(err error) error {
	if err == nil {
		return nil
	}

	// Fallback for unrecognized wrapping
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

func ToSnakeCase(str string) string {
	// Regular expression to match uppercase letters or sequences of them
	re := regexp.MustCompile("([a-z0-9])([A-Z])")
	// Replace with lowercase and underscore
	snake := re.ReplaceAllString(str, "${1}_${2}")
	// Special case for sequences of uppercase letters
	snake = regexp.MustCompile("([A-Z])([A-Z][a-z])").ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
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
