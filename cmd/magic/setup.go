package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/magic/configmanager"
	"github.com/malekradhouane/magic/middleware"
	"github.com/malekradhouane/magic/pkg/gatekeeper"
	"github.com/malekradhouane/magic/pkg/gatekeeper/oidc"
	"github.com/malekradhouane/magic/pkg/mailer"
	"github.com/malekradhouane/magic/store"
	"github.com/malekradhouane/magic/store/postgres"
	"github.com/malekradhouane/magic/store/types"
)

type (
	ResourcesRegistry struct {
		cman       *configmanager.ManagerWithKoanf
		logger     *logrus.Logger
		loggerSlog *slog.Logger
		mailer     mailer.Mailer
		http       struct {
			ginEngine            *gin.Engine
			ginRouterAPI         *gin.RouterGroup
			ginJwt               *middleware.GinJWT
			ginAuthMiddleware    *jwt.GinJWTMiddleware
			hybridAuthMiddleware gin.HandlerFunc
		}

		repository struct {
			user types.UserStore
		}
		gatekeeper struct {
			casbin             *casbin.Enforcer
			sessionStore       gatekeeper.IdentityStore
			gatekeeperContract gatekeeper.GatekeeperContract
			verifier           *gatekeeper.Verifier
		}

		stores struct {
			postgresClient *postgres.Client
		}
		services    struct{}
		controllers struct{}
		feature     struct{}
		// List of components to close on shutdown.
		closers []interface{ Close(context.Context) error }
	}
)

type SetupFn func() error

// Setup Setups the resource registry
func (rr *ResourcesRegistry) Setup() error {
	if rr.logger == nil {
		rr.logger = logrus.New()
	}

	setupOrder := []struct {
		fn   SetupFn
		desc string
	}{
		{rr.setupConfigManager, "configuration"},
		{rr.setupLogger, "logger"},
		{rr.setupStartup, "startup"},
		{rr.setupStoragePostgreSQL, "storage using PostgreSQL"},
		{rr.setupRepository, "repositories"},
		{rr.setupMailer, "mailer"},
		{rr.setupGinRouter, "gin router"},
	}

	for _, setup := range setupOrder {
		rr.logger.Info("setup: " + setup.desc + " ...")

		if err := setup.fn(); err != nil {
			rr.logger.Info(setup.desc + ": failed")
			return err
		}

		rr.logger.Info(setup.desc + ": ok")
	}

	return nil
}

// Shutdown free all allocated resources by Setup() and os.Exit()
func (rr *ResourcesRegistry) Shutdown(appErr error) {
	logger := rr.logger
	if logger == nil {
		logger = logrus.New()
	}

	var err error
	ctx := context.Background()
	for _, closer := range slices.Backward(rr.closers) {
		err = errors.Join(err, closer.Close(ctx))
	}

	if err != nil {
		logger.Error(err)
	}

	if appErr != nil {
		logger.Error(appErr)
		os.Exit(1)
	}

	os.Exit(0)
}

func (rr *ResourcesRegistry) setupConfigManager() error {
	const (
		EnvConfigRootDir = "MAGIC_CONFIG_ROOT_DIR"
		EnvEnvironment   = "ENVIRONMENT"
	)

	configRootDir := os.Getenv(EnvConfigRootDir)
	env := os.Getenv(EnvEnvironment)

	cman, err := configmanager.DefaultManagerWithKonf().
		WithConfigRoot(configRootDir).
		WithEnvironment(env).
		Build()
	if err != nil {
		return err
	}
	rr.cman = cman

	return nil
}

func (rr *ResourcesRegistry) setupLogger() error {
	cnf := rr.cman.Magic().Logging
	newLogger := logrus.New()

	if logLevel, err := logrus.ParseLevel(cnf.Level); err == nil {
		if logLevel == logrus.DebugLevel {
			newLogger.SetLevel(logLevel)
			newLogger.Debug("debug log enabled")
		}
	}

	if cnf.Formatter == "json" {
		newLogger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		newLogger.SetFormatter(&logrus.TextFormatter{})
	}

	if cnf.Verbose {
		newLogger.SetReportCaller(true) // show the logrus line and file
		newLogger.WithFields(logrus.Fields{
			"app": "Magic",
		})
	}

	rr.logger = newLogger

	return nil
}

func (rr *ResourcesRegistry) setupStartup() error {
	execPreList := rr.cman.Magic().Startup.ExecPre

	if cwd, err := os.Getwd(); err == nil {
		rr.logger.Info("working directory=", cwd)
	} else {
		return err
	}

	for _, execPre := range execPreList {
		if execPre.Disabled {
			continue
		}

		rr.logger.Info("startup execPre", " name=", execPre.Name, " description=", execPre.Desc)
		cmd := exec.Command(execPre.Command, execPre.Args...)
		cmd.Dir = execPre.WorkDir
		cmd.Env = rr.mergeWithEnviron(execPre.Env)
		output, err := cmd.CombinedOutput()
		rr.logger.Info("startup execPre", " name=", execPre.Name, " rc=", cmd.ProcessState.ExitCode())
		if err != nil {
			rr.logger.Error(err.Error(), " output=", string(output))
			if !execPre.ContinueOnFailure {
				return err
			}
		}
	}

	return nil
}

func (rr *ResourcesRegistry) mergeWithEnviron(userEnv []string) []string {
	// Build an environment map with current os.Environ().

	envMap := map[string]string{}
	for _, keyVal := range os.Environ() {
		parts := strings.SplitN(keyVal, "=", 2)
		envMap[parts[0]] = parts[1]
	}

	// Add/Replace var in map with user configured env.

	for _, keyVal := range userEnv {
		parts := strings.SplitN(keyVal, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = os.ExpandEnv(parts[1])
		}
	}

	// Recreate a shell-style environment

	newEnv := make([]string, 0, len(envMap))
	for key, val := range envMap {
		newEnv = append(newEnv, key+"="+val)
	}

	return newEnv
}

func (rr *ResourcesRegistry) setupRepository() error {

	if repo, err := postgres.NewUserStore(); err != nil {
		return err
	} else {
		rr.repository.user = repo
		rr.closers = append(rr.closers, &ctxCloser{rr.repository.user})
	}

	return nil
}

func (rr *ResourcesRegistry) setupMailer() error {
	emailConfig := rr.cman.Magic().Email
	// Get default values from .env
	emailConfig.Mailjet.APIKeyPrivate = os.Getenv("MAILJET_API_KEY_PRIVATE")

	// Only setup mailer if credentials are provided
	if emailConfig.Mailjet.APIKeyPublic == "" || emailConfig.Mailjet.APIKeyPrivate == "" {
		rr.logger.Warn("Mailjet credentials not provided, email functionality will be disabled")
		return nil
	}

	mailjetConfig := mailer.Config{
		APIKeyPublic:  emailConfig.Mailjet.APIKeyPublic,
		APIKeyPrivate: emailConfig.Mailjet.APIKeyPrivate,
		DefaultFrom: struct {
			Name  string
			Email string
		}{
			Name:  emailConfig.Mailjet.FromName,
			Email: emailConfig.Mailjet.FromEmail,
		},
	}

	if mailjetConfig.DefaultFrom.Name == "" {
		mailjetConfig.DefaultFrom.Name = "Magic"
	}
	if mailjetConfig.DefaultFrom.Email == "" {
		mailjetConfig.DefaultFrom.Email = "noreply@magic.fr"
	}

	mailjetMailer, err := mailer.NewMailjet(mailjetConfig)
	if err != nil {
		return fmt.Errorf("failed to create mailjet mailer: %w", err)
	}

	rr.mailer = mailjetMailer
	rr.logger.Info("Mailer configured successfully")

	return nil
}

func (rr *ResourcesRegistry) setupGinRouter() error {

	rr.http.ginEngine = gin.New()

	if ginJWT, err := middleware.NewGinJwt(rr.cman, rr.repository.user); err != nil {
		return err
	} else {
		rr.http.ginJwt = ginJWT
	}

	rr.http.ginEngine.Static(PREFIX, FOLDER)
	rr.http.ginEngine.Use(rr.http.ginJwt.LoggerWithUsername())
	rr.http.ginEngine.Use(gin.Recovery())
	rr.http.ginEngine.Use(cors.New(cors.Config{
		// AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "PUT", "POST", "DELETE", "PATCH", "OPTIONS", "HEAD"},
		AllowHeaders:     []string{"Origin", "Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		AllowAllOrigins:  true,
		MaxAge:           12 * time.Hour,
	}))

	rr.http.ginRouterAPI = rr.http.ginEngine.Group("/api")

	if authMiddleware, err := jwt.New(rr.http.ginJwt.MiddlewareHandler()); err != nil {
		return err
	} else {
		rr.http.ginAuthMiddleware = authMiddleware
	}

	return nil
}

func (rr *ResourcesRegistry) setupStoragePostgreSQL() error {
	dbConfig := rr.cman.Magic().Storage.PostgreSQL

	dbConfig.MagicPass = os.Getenv("DB_PASSWORD")
	dbConfig.MagicLogin = os.Getenv("DB_LOGIN")
	dbConfig.MagicDB = os.Getenv("DB_NAME")
	dbConfig.Host = os.Getenv("DB_HOST")

	client := postgres.NewClient(
		postgres.ConnParams{
			Host:         dbConfig.Host,
			Port:         dbConfig.Port,
			Database:     dbConfig.MagicDB,
			UserName:     dbConfig.MagicLogin,
			UserPassword: dbConfig.MagicPass,
		})

	if err := client.Initialise(); err != nil {
		client.Close()
		return fmt.Errorf("postgresClient.Initialise err: %w", err)
	}

	// ping the database to ensure connectivity
	if err := client.Ping(); err != nil {
		client.Close()
		return fmt.Errorf("InitialiseStoragePostgres: unable to ping the database: %w", err)
	}
	opts := &store.Options{
		// Uncomment to request creation of one or more mock data stores
	}
	if err := store.CreatePostgresStores(opts); err != nil {
		client.Close()
		return err
	}

	rr.stores.postgresClient = client

	closer := &ctxCloser{closer: rr.stores.postgresClient}
	rr.closers = append(rr.closers, closer)

	return nil
}

func (rr *ResourcesRegistry) setupEarlyService() error {
	return nil
}

func (rr *ResourcesRegistry) setupGatekeeper() error {
	// get postgres connection string
	dbConfig := rr.cman.Magic().Storage.PostgreSQL
	// Allow overriding SSL mode via environment variable. Defaults to "disable" for local dev.
	sslmode := os.Getenv("MAGIC_PG_SSLMODE")
	if sslmode == "" {
		sslmode = "require"
	}

	dsn := fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s?sslmode=%s",
		dbConfig.MagicLogin, dbConfig.MagicPass, dbConfig.Host, dbConfig.Port, dbConfig.MagicDB, sslmode,
	)
	a, err := gormadapter.NewAdapter("postgres", dsn, true)
	if err != nil {
		return fmt.Errorf("failed to create adapter: %w", err)
	}

	// get enforcer according to given rbac model configuration file and adapter
	// TODO : get this from configuration api instead of having it hard coded
	enforcer, err := casbin.NewEnforcer("config/rbac_model.conf", a)
	if err != nil {
		return fmt.Errorf("failed to create enforcer: %w", err)
	}
	// Load endpoint from DB dynamically
	err = enforcer.LoadPolicy()
	if err != nil {
		return fmt.Errorf("failed to load policy: %w", err)
	}

	rr.gatekeeper.casbin = enforcer

	fmt.Println("Casbin enforcer loaded", enforcer)

	oidcConfig := rr.cman.Magic().Auth.OIDC
	cookie := rr.cman.Magic().Auth.Cookie
	oidcClaimsConfig := oidc.ClaimsConfig{
		UsernameClaim: oidcConfig.Claims.UsernameClaim,
		RoleClaim:     oidcConfig.Claims.RoleClaim,
	}
	if oidcConfig.Enabled {
		// Create OIDC provider configuration
		provider, err := oidc.NewDiscoveryOIDCProvider(
			context.Background(),
			oidcConfig.ProviderName,
			oidcConfig.IssuerURL,
			oidcConfig.ClientID,
			oidcConfig.ClientSecret,
			oidcConfig.RedirectURL,
			oidcConfig.Scopes,
			oidcClaimsConfig,
			oidcConfig.ExtraAuthParams,
			false,
			false,
		)
		if err != nil {
			return fmt.Errorf("failed to create OIDC provider: %w", err)
		}

		// Create token verifier
		verifier := gatekeeper.NewVerifier(provider)
		rr.gatekeeper.verifier = verifier

		cookieConfig := gatekeeper.CookieConfig{
			Domain:        cookie.Domain,
			Secure:        cookie.Secure,
			HTTPOnly:      cookie.HTTPOnly,
			SameSite:      cookie.SameSite,
			SessionTTL:    time.Duration(cookie.SessionTTL) * time.Minute,
			SessionSecret: cookie.SessionSecret,
		}

		claimsConfig := gatekeeper.ClaimsConfig{
			UsernameClaim: oidcConfig.Claims.UsernameClaim,
			RoleClaim:     oidcConfig.Claims.RoleClaim,
		}

		gatekeeperConfig := gatekeeper.Config{
			StateTTL:           10 * time.Minute,
			DefaultRedirectURL: oidcConfig.RedirectURL,
			AuthExtra:          make(map[string]string),
			Scopes:             oidcConfig.Scopes,
			SessionTTL:         time.Duration(cookie.SessionTTL) * time.Minute,
		}

		sessionStore := gatekeeper.NewIdentityStore(time.Hour, cookie.SessionSecret, "magic_session", claimsConfig, cookieConfig)
		rr.gatekeeper.sessionStore = sessionStore

		// Create Gatekeeper feature
		gkParams := gatekeeper.NewGateKeeperParams{
			Logger:        rr.loggerSlog.With("component", "gatekeeper"),
			GinRouter:     rr.http.ginEngine,
			Enforcer:      enforcer,
			OIDCProvider:  provider,
			StateStore:    gatekeeper.NewMemoryStateStore(),
			Config:        gatekeeperConfig,
			IdentityStore: sessionStore,
			OIDCVerifier:  verifier,
			CookieConfig:  cookieConfig,
			BaseURL:       oidcConfig.PublicBaseURL,
		}

		gkFeature, err := gatekeeper.NewGateKeeper(gkParams)
		if err != nil {
			return fmt.Errorf("failed to create gatekeeper feature: %w", err)
		}

		rr.gatekeeper.gatekeeperContract = gkFeature

		hybridAuthMiddleware := middleware.DualTokenMiddleware(rr.http.ginAuthMiddleware, rr.gatekeeper.verifier, rr.gatekeeper.sessionStore, rr.http.ginJwt)
		rr.http.hybridAuthMiddleware = hybridAuthMiddleware

		rr.closers = append(rr.closers, gkFeature)
	} else {
		rr.logger.Warn("OIDC authentication is disabled or mode != oidc")
	}

	return nil
}

type ctxCloser struct {
	closer interface{ Close() error }
}

func (x *ctxCloser) Close(_ context.Context) error {
	return x.closer.Close()
}

type ctxCloserNil struct {
	closer interface{ Close() }
}

func (x *ctxCloserNil) Close(_ context.Context) error {
	x.closer.Close()

	return nil
}
