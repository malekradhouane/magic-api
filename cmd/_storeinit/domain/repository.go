package domain

import "context"

// Magic repository

const (
	ActionMigrateUp    = "up"
	ActionMigrateDown  = "down"
	ActionMigrateForce = "force"
)

type (
	MigrateRequest struct {
		Action string // CmdMigrate*
	}

	MigrateResponse struct {
		Repo     RepositoryContract
		NoChange bool
		Err      error
	}

	MigrationStatusResponse struct {
		Repo          RepositoryContract
		SchemaVersion int // -1 = no migration yet applied
		Dirty         bool
		Err           error
	}
)

type RepositoryContract interface {
	Migrate(ctx context.Context, req MigrateRequest) MigrateResponse
	MigrationStatus(ctx context.Context) MigrationStatusResponse
	Name() string
	Driver() string
	Close(ctx context.Context) error
}
