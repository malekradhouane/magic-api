package postgres

import "gorm.io/gorm"

// withTransaction runs fn inside a GORM transaction. If fn returns an error
// the transaction is rolled back. A panic inside fn is rolled back and then
// re-raised so the caller's deferred handlers (and pprof / sentry) still see it.
func withTransaction(db *gorm.DB, fn func(tx *gorm.DB) error) error {
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r) // re-raise so observability tooling can capture it
		}
	}()

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}
