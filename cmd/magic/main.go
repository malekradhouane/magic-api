package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // required to embed timezone data directly into the Go binary.

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"     // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger" // gin-swagger middleware

	"github.com/malekradhouane/magic/auth"
	"github.com/malekradhouane/magic/docs"
	"github.com/malekradhouane/magic/handler"
	"github.com/malekradhouane/magic/middleware"
	"github.com/malekradhouane/magic/pkg/sse"
	"github.com/malekradhouane/magic/service"
	"github.com/malekradhouane/magic/store"
	"github.com/malekradhouane/magic/utils/httpresp"
)

const (
	PREFIX = "/static"
	FOLDER = "uploads"
)

// @title Magic - Inventory solution API
// @version 1.0
// @description Here is our solution documentation and testing portal of provided functionalities to interact with our hypervisor tool.
// @termsOfService https://www.magic.com/terms/

// @contact.name API Support
// @contact.url https://www.magic.fr/support
// @contact.email malek.radhouen@gmail.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:5001
// @BasePath /api
// @query.collection.format multi

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

// @x-extension-openapi {"example": "value on a json format"}
func main() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		// .env file is optional, continue without it
		fmt.Println("No .env file found, using environment variables")
	}

	rr := new(ResourcesRegistry)
	if err := rr.Setup(); err != nil {
		rr.Shutdown(err) // Will exit
	}

	auth.Init()
	itconfig := rr.cman.Magic()

	errorChan := make(chan error, 1)

	// creates buffered error channel for stopping main process if an error occurs
	go func(c chan error) {
		for err := range c {
			if err == nil {
				rr.logger.Warn("unexpected nil error")
				continue
			}
			err = fmt.Errorf("error starting monitoring : %w", err)
			rr.Shutdown(err)
		}
	}(errorChan)

	// init HTTP router
	r := rr.http.ginEngine
	authMiddleware := rr.http.ginAuthMiddleware
	// Security headers on every response (HSTS, CSP, X-Frame-Options, …).
	r.Use(middleware.SecurityHeaders())
	// Cap multipart uploads (CSV imports etc.) to 16 MiB in memory; the rest
	// spills to /tmp. Defaults to 32 MiB which is too generous for our needs.
	r.MaxMultipartMemory = 16 << 20
	// Ping test
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// Route not found
	r.NoRoute(authMiddleware.MiddlewareFunc(), func(c *gin.Context) {
		claims := jwt.ExtractClaims(c)
		rr.logger.Printf("NoRoute claims: %#v\n", claims)
		httpresp.NewErrorMessage(c, http.StatusNotFound, "No resource is found.")
	})

	ginJWT := rr.http.ginJwt

	// Rate limiters used across the auth surface.
	//   ipAuthLimiter:    10 attempts / minute / IP — covers /login, /authenticate.
	//   emailAuthLimiter: 5 attempts / 15 minutes / email — defeats credential
	//                     stuffing on a known account.
	//   registerLimiter:  5 attempts / hour / IP — slows down account farming
	//                     and bcrypt-based DoS via POST /users.
	//   guestOrderLimiter: 5 attempts / 15 minutes / IP — slows down enumeration
	//                     of guest order lookups by phone.
	ipAuthLimiter := middleware.NewRateLimiter(10, 1*time.Minute)
	emailAuthLimiter := middleware.NewRateLimiter(5, 15*time.Minute)
	registerLimiter := middleware.NewRateLimiter(5, 1*time.Hour)
	guestOrderLimiter := middleware.NewRateLimiter(5, 15*time.Minute)

	loginRL := middleware.LoginRateLimit(ipAuthLimiter, emailAuthLimiter)
	genericAuthRL := middleware.LoginRateLimit(ipAuthLimiter, nil)
	registerRL := registerLimiter.Middleware()
	guestOrderRL := guestOrderLimiter.Middleware()

	// Login endpoint — combined IP + email rate limit.
	r.POST("/login", loginRL, authMiddleware.LoginHandler)

	api := r.Group("/api")
	userService := service.NewUserService(store.Users(), rr.logger)
	mailjetCfg := rr.cman.Magic().Email.Mailjet
	applyMailjetEnvOverrides(&mailjetCfg)
	authService := service.NewAuthService(
		store.Users(),
		rr.logger,
		rr.mailer,
		mailjetCfg.FromName,
		mailjetCfg.FromEmail,
	)

	// E-commerce services
	categoryService := service.NewCategoryService(store.Categories(), rr.logger)
	// Ensure the default product-type taxonomy exists (idempotent, runs each boot).
	if err := categoryService.SeedDefaults(context.Background()); err != nil {
		rr.logger.WithError(err).Warn("failed to seed default categories")
	}
	productService := service.NewProductService(store.Products(), rr.logger)
	promoService := service.NewPromoService(store.Promos(), rr.logger)
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	addressService := service.NewAddressService(store.Addresses(), rr.logger)
	settingsService := service.NewSettingsService(store.Settings(), rr.logger)
	orderService := service.NewOrderService(
		store.Orders(),
		store.Products(),
		promoService,
		store.Users(),
		addressService,
		settingsService,
		rr.mailer,
		mailjetCfg.FromName,
		mailjetCfg.FromEmail,
		frontendURL,
		rr.logger,
	)

	// Real-time admin notifications (SSE). The hub fans out events to every
	// connected admin; the order service pushes a "new_order" event on create.
	notificationHub := sse.NewHub()
	notificationService := service.NewNotificationService(notificationHub, rr.logger)
	orderService.SetNotifier(notificationService)

	// Sets up auth routes
	authCtrl, err := handler.NewController(rr.cman, authMiddleware, ginJWT, authService, loginRL, genericAuthRL)
	if err != nil {
		rr.Shutdown(err)
	}
	authCtrl.SetupRoutes(api)

	// Sets up user routes
	userHandler := handler.NewUserHandler(userService, authService, authMiddleware.MiddlewareFunc(), registerRL, rr.cman)
	userHandler.SetupUsersRoutes(api)

	// E-commerce routes
	categoryHandler := handler.NewCategoryHandler(categoryService, authMiddleware.MiddlewareFunc())
	categoryHandler.SetupRoutes(api)

	productHandler := handler.NewProductHandler(productService, categoryService, authMiddleware.MiddlewareFunc())
	productHandler.SetupRoutes(api)

	orderHandler := handler.NewOrderHandler(
		orderService,
		authMiddleware.MiddlewareFunc(),
		middleware.OptionalJWTMiddleware(authMiddleware),
		guestOrderRL,
	)
	orderHandler.SetupRoutes(api)

	notificationHandler := handler.NewNotificationHandler(notificationHub, authMiddleware.MiddlewareFunc())
	notificationHandler.SetupRoutes(api)

	addressHandler := handler.NewAddressHandler(addressService, authMiddleware.MiddlewareFunc())
	addressHandler.SetupRoutes(api)

	promoHandler := handler.NewPromoHandler(promoService, authMiddleware.MiddlewareFunc())
	promoHandler.SetupRoutes(api)

	// Dashboard statistics (admin only)
	statsService := service.NewStatsService(store.Stats(), settingsService, rr.logger)
	statsHandler := handler.NewStatsHandler(statsService, authMiddleware.MiddlewareFunc())
	statsHandler.SetupRoutes(api)

	// Application settings (admin only)
	settingsHandler := handler.NewSettingsHandler(settingsService, authMiddleware.MiddlewareFunc())
	settingsHandler.SetupRoutes(api)

	// Uploads (presigned URLs to Cloudflare R2). r2Client may be nil if R2
	// is not configured; the handler will then return 503 to clients.
	uploadHandler := handler.NewUploadHandler(rr.stores.r2Client, authMiddleware.MiddlewareFunc())
	uploadHandler.SetupRoutes(api)

	// Serve swagger documentation (only in dev mode)
	if os.Getenv("MODE") == "dev" || os.Getenv("GIN_MODE") != "release" {
		r.GET("/swagger/*any", setDocumentationInfo, ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// Starts up server with hardened timeouts to mitigate Slowloris-style
	// attacks and resource exhaustion. Values are intentionally conservative
	// for a JSON API; bump WriteTimeout when serving large file downloads.
	httpConfig := itconfig.HttpServer
	listenAddr := fmt.Sprintf("%s:%d", httpConfig.Listen, httpConfig.Port)
	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}
	go func() {
		if httpConfig.TLS {
			rr.logger.Println("TLS ON", listenAddr)
			err = srv.ListenAndServeTLS(httpConfig.CertFile, httpConfig.KeyFile)
		} else {
			rr.logger.Println("TLS OFF", listenAddr)
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			rr.Shutdown(err)
		}
	}()

	// Accept user break
	rr.logger.Info("Magic is running ...")
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	<-done
	close(done)

	rr.Shutdown(nil)
}

func setDocumentationInfo(c *gin.Context) {
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.Host = c.Request.Host
}
