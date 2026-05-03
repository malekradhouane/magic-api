package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"storeinit/app"
	"storeinit/config"
	"storeinit/domain"
	"storeinit/infra/controllers/cli"
	repo_pg "storeinit/infra/repo/pg"
)

const (
	EnvConfigPath = "STOREINIT_CONFIG"
)

type Resources struct {
	cman     config.ManagerContract
	repoList []domain.RepositoryContract
	app      *app.Application
	cli      *cli.Controller
}

func main() {
	// Note: .env loading is handled by the parent process
	// Environment variables are passed explicitly via execPre config

	rss, err := Setup() // Create ressources container
	if err != nil {
		Shutdown(rss, err)
	}

	err = rss.cli.Run(GitTag, rss.cman)

	Shutdown(rss, err)
}

func Setup() (*Resources, error) {
	// log.SetFlags(log.LstdFlags | log.Llongfile)

	rss := new(Resources)

	// Create configuration manager.

	cman, err := config.DefaultManager().
		WithConfigPath(os.Getenv(EnvConfigPath)).
		Build()
	if err != nil {
		return rss, err
	} else {
		rss.cman = cman
	}

	// Create the list of repositories to handle from configuration.

	for i, repoConfig := range rss.cman.Configuration().Storages {
		if !repoConfig.Enabled {
			continue
		}

		switch driver := strings.ToLower(repoConfig.Driver); driver {
		case config.DriverPostgreSQL:
			// Build database URI from environment variables if not provided

			host := os.Getenv("DB_HOST")
			port := os.Getenv("DB_PORT")
			user := os.Getenv("DB_LOGIN")
			password := os.Getenv("DB_PASSWORD")
			dbname := os.Getenv("DB_NAME")
			dbURI := ""
			// Debug logging
			fmt.Printf("DEBUG: DB_HOST=%s, DB_PORT=%s, DB_LOGIN=%s, DB_PASSWORD=***, DB_NAME=%s\n",
				host, port, user, dbname)

			// Allow overriding SSL mode via environment variable. Defaults to "require" for production.
			sslmode := os.Getenv("MAGIC_PG_SSLMODE")
			if sslmode == "" {
				sslmode = "require"
			}

			if host != "" && user != "" && password != "" && dbname != "" {
				if port == "" {
					port = "5432"
				}
				dbURI = fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, dbname, sslmode)
				fmt.Printf("DEBUG: Built dbURI=%s\n", dbURI)
			} else {
				fmt.Printf("DEBUG: Missing required environment variables\n")
			}

			adapter, err := repo_pg.NewAdapterBuilder().
				SetName(repoConfig.Name).
				SetDatabaseURI(dbURI).
				SetSchemaFolder(repoConfig.SchemaFolder).
				SetForceSchemaVersion(repoConfig.ForceSchemaVersion).
				Build()
			if err != nil {
				return rss, err
			}

			rss.repoList = append(rss.repoList, adapter)

		default:
			return rss, fmt.Errorf("unknown driver: %s for repo #%d (%s)", driver, i, repoConfig.Name)
		}
	}

	// Create app

	app, err := app.NewApplicationBuilder().
		SetRepositories(rss.repoList...).
		Build()
	if err != nil {
		return rss, err
	}
	rss.app = app

	// Create CLI handler

	cli, err := cli.NewControllerBuilder().
		SetApplication(rss.app).
		Build(context.TODO())
	if err != nil {
		return rss, err
	}
	rss.cli = cli

	return rss, nil
}

func Shutdown(rss *Resources, apperr error) {
	if rss != nil {

		if rss.cli != nil {
			if err := rss.cli.Close(context.Background()); err != nil {
				slog.Error(err.Error())
			}
		}

		if rss.app != nil {
			if err := rss.app.Close(context.TODO()); err != nil {
				slog.Error(err.Error())
			}
		}

		for _, repo := range rss.repoList {
			if err := repo.Close(context.TODO()); err != nil {
				slog.Error(err.Error())
			}
		}

	}

	if apperr != nil {
		slog.Error(apperr.Error())
		os.Exit(1)
	}

	os.Exit(0)
}

var GitTag string // Set at compilation time
