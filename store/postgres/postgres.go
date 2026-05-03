package postgres

import (
	"fmt"
	"net/url"
	"os"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Client is the main contact point to interact with a remote PostgreSQL server.
// After creation (see New functions), Client instances must be initialised (see Initialise)
type Client struct {
	ConnParams

	// Holds the *gorm.DB session
	session *Session
}

// Initialise instantiates a GORM session and connects to the remote host
// specified in ConnParams.
//
// Important: make sure to prepare a call to Close() when done with a Client
func (c *Client) Initialise() error {
	log := logrus.New()

	portAsString := fmt.Sprintf("%d", c.ConnParams.Port)

	// Allow overriding SSL mode via environment variable. Defaults to "disable" for local dev.
	sslmode := os.Getenv("MAGIC_PG_SSLMODE")
	if sslmode == "" {
		sslmode = "require"
	}

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
		c.ConnParams.Host,
		c.ConnParams.UserName,
		url.QueryEscape(c.ConnParams.UserPassword),
		c.ConnParams.Database,
		portAsString,
		sslmode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		obfuscatedDSN := dsn
		log.Errorf("Failed to connect to PostgreSQL: %v with DSN: %s", err, obfuscatedDSN)

		return fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Infoln("PostgreSQL Client: GORM session created.")

	config := &Config{
		AccessControlEnabled: c.ConnParams.AuthWithUserAndPassword,
		Database: Database{
			Name:     c.ConnParams.Database,
			Username: c.ConnParams.UserName,
			Password: c.ConnParams.UserPassword,
		},
		Host: c.ConnParams.Host,
		Port: portAsString,
	}

	c.session = &Session{
		db:     db,
		config: config,
	}

	if err := c.Ping(); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	log.Infoln("PostgreSQL Client: Session connected")
	return nil
}

// Ping checks the connection to PostgreSQL.
func (c *Client) Ping() error {
	sqlDB, err := c.session.db.DB()
	if err != nil {
		return fmt.Errorf("failed to retrieve underlying SQL DB: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}
	return nil
}

// Session returns the GORM session.
func (c *Client) Session() *Session {
	return c.session
}

// Close terminates the session.
func (c *Client) Close() error {
	log := logrus.New()
	if c == nil {
		return fmt.Errorf("Client not created")
	}
	if c.session == nil {
		return fmt.Errorf("Client not initialised")
	}

	sqlDB, err := c.session.db.DB()
	if err != nil {
		return fmt.Errorf("failed to retrieve underlying SQL DB: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close PostgreSQL connection: %w", err)
	}

	log.Infoln("PostgreSQL Client: Session closed")
	return nil
}
