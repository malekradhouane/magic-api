// Command createadmin creates an admin user in the database.
// This is a CLI tool for bootstrapping the first superadmin user.
// After the first superadmin is created, subsequent admins can be created via the API.
//
// Usage:
//
//	# Create a superadmin (first user)
//	go run ./cmd/createadmin --email admin@magic.tn --password secret123 --username admin
//	make create-admin EMAIL=admin@magic.tn PASSWORD=secret123
//
//	# Or use environment variables
//	ADMIN_EMAIL=admin@magic.tn ADMIN_PASSWORD=secret123 go run ./cmd/createadmin
//
// Security Notes:
//   - Password must be at least 8 characters
//   - Email is normalized to lowercase
//   - If user already exists, it will be upgraded to admin role
//   - The user is created with email_verified=true and is_active=true
//
// The PostgreSQL connection is resolved via config/magic.yaml (port) and
// environment variables DB_HOST, DB_LOGIN, DB_PASSWORD, DB_NAME (.env).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/magic/configmanager"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/store"
	"github.com/malekradhouane/magic/store/postgres"
	"github.com/malekradhouane/magic/utils/encrypt"
)

func main() {
	var (
		email     string
		password  string
		username  string
		firstName string
		lastName  string
	)
	flag.StringVar(&email, "email", "", "admin email (required)")
	flag.StringVar(&password, "password", "", "admin password (required, min 8 chars)")
	flag.StringVar(&username, "username", "", "admin username (defaults to email)")
	flag.StringVar(&firstName, "first-name", "", "admin first name")
	flag.StringVar(&lastName, "last-name", "", "admin last name")
	flag.Parse()

	// Allow env-var overrides so `make create-admin` can pass values.
	if v := os.Getenv("ADMIN_EMAIL"); v != "" && email == "" {
		email = v
	}
	if v := os.Getenv("ADMIN_PASSWORD"); v != "" && password == "" {
		password = v
	}
	if v := os.Getenv("ADMIN_USERNAME"); v != "" && username == "" {
		username = v
	}

	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	if email == "" || password == "" {
		logger.Fatal("--email and --password are required")
	}
	if len(password) < 8 {
		logger.Fatal("password must be at least 8 characters")
	}

	email = strings.ToLower(strings.TrimSpace(email))
	if username == "" {
		username = email
	}

	if err := godotenv.Load(); err != nil {
		logger.Warn("no .env file found, using environment variables")
	}

	client, err := setupPostgres()
	if err != nil {
		logger.WithError(err).Fatal("PostgreSQL connection failed")
	}
	defer func() {
		if err := client.Close(); err != nil {
			logger.WithError(err).Warn("failed to close database connection")
		}
	}()

	if err := store.CreatePostgresStores(nil); err != nil {
		logger.WithError(err).Fatal("failed to initialize stores")
	}

	ctx := context.Background()
	userStore := store.Users()

	// Check if admin already exists
	existing, _ := userStore.GetUserByEmail(ctx, email)
	if existing != nil {
		logger.WithField("email", email).Info("user already exists, updating role to admin")
		if _, err := userStore.UpdateUserFields(ctx, existing.ID.String(), map[string]interface{}{
			"role": "admin",
		}); err != nil {
			logger.WithError(err).Fatal("failed to update existing user role")
		}
		logger.WithFields(logrus.Fields{"email": email, "id": existing.ID}).Info("admin role set successfully")
		return
	}

	hashedPassword, err := encrypt.Hash(password)
	if err != nil {
		logger.WithError(err).Fatal("failed to hash password")
	}

	user := &interfaces.User{
		Email:         email,
		PasswordHash:  string(hashedPassword),
		Provider:      "password",
		Role:          "admin",
		Username:      username,
		FirstName:     firstName,
		LastName:      lastName,
		EmailVerified: true,
		IsActive:      true,
	}

	created, err := userStore.CreateUser(ctx, user, "", "admin")
	if err != nil {
		logger.WithError(err).Fatal("failed to create admin user")
	}

	logger.WithFields(logrus.Fields{
		"id":    created.ID,
		"email": created.Email,
		"role":  created.Role,
	}).Info("admin user created successfully")
}

// setupPostgres connects to PostgreSQL using config + env vars.
func setupPostgres() (*postgres.Client, error) {
	configRootDir := os.Getenv("MAGIC_CONFIG_ROOT_DIR")
	env := os.Getenv("ENVIRONMENT")

	cman, err := configmanager.DefaultManagerWithKonf().
		WithConfigRoot(configRootDir).
		WithEnvironment(env).
		Build()
	if err != nil {
		return nil, fmt.Errorf("loading configuration: %w", err)
	}

	dbConfig := cman.Magic().Storage.PostgreSQL

	client := postgres.NewClient(postgres.ConnParams{
		Host:         os.Getenv("DB_HOST"),
		Port:         dbConfig.Port,
		Database:     os.Getenv("DB_NAME"),
		UserName:     os.Getenv("DB_LOGIN"),
		UserPassword: os.Getenv("DB_PASSWORD"),
	})

	if err := client.Initialise(); err != nil {
		client.Close()
		return nil, fmt.Errorf("client initialization: %w", err)
	}
	if err := client.Ping(); err != nil {
		if closeErr := client.Close(); closeErr != nil {
			return nil, fmt.Errorf("database ping failed: %w (close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("database ping: %w", err)
	}
	return client, nil
}
