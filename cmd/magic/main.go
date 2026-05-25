package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata" // required to embed timezone data directly into the Go binary.

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"     // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger" // gin-swagger middleware

	"github.com/malekradhouane/magic/auth"
	"github.com/malekradhouane/magic/docs"
	"github.com/malekradhouane/magic/handler"
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

	// Login endpoint
	r.POST("/login", authMiddleware.LoginHandler)

	api := r.Group("/api")
	userService := service.NewUserService(store.Users(), rr.logger)
	authService := service.NewAuthService(store.Users(), rr.logger, rr.mailer)

	// E-commerce services
	categoryService := service.NewCategoryService(store.Categories(), rr.logger)
	productService := service.NewProductService(store.Products(), rr.logger)
	promoService := service.NewPromoService(store.Promos(), rr.logger)
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	orderService := service.NewOrderService(
		store.Orders(),
		store.Products(),
		promoService,
		store.Users(),
		rr.mailer,
		frontendURL,
		rr.logger,
	)

	// Sets up auth routes
	authCtrl, err := handler.NewController(rr.cman, authMiddleware, ginJWT, authService)
	if err != nil {
		rr.Shutdown(err)
	}
	authCtrl.SetupRoutes(api)

	// Sets up user routes
	userHandler := handler.NewUserHandler(userService, authService, authMiddleware.MiddlewareFunc(), rr.cman)
	userHandler.SetupUsersRoutes(api)

	// E-commerce routes
	categoryHandler := handler.NewCategoryHandler(categoryService, authMiddleware.MiddlewareFunc())
	categoryHandler.SetupRoutes(api)

	productHandler := handler.NewProductHandler(productService, authMiddleware.MiddlewareFunc())
	productHandler.SetupRoutes(api)

	orderHandler := handler.NewOrderHandler(orderService, authMiddleware.MiddlewareFunc())
	orderHandler.SetupRoutes(api)

	promoHandler := handler.NewPromoHandler(promoService, authMiddleware.MiddlewareFunc())
	promoHandler.SetupRoutes(api)

	// Uploads (presigned URLs to Cloudflare R2). r2Client may be nil if R2
	// is not configured; the handler will then return 503 to clients.
	uploadHandler := handler.NewUploadHandler(rr.stores.r2Client, authMiddleware.MiddlewareFunc())
	uploadHandler.SetupRoutes(api)

	// Serve swagger documentation
	r.GET("/swagger/*any", setDocumentationInfo, ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Starts up server
	httpConfig := itconfig.HttpServer
	listenAddr := fmt.Sprintf("%s:%d", httpConfig.Listen, httpConfig.Port)
	go func() {
		if httpConfig.TLS {
			rr.logger.Println("TLS ON", listenAddr)
			err = http.ListenAndServeTLS(listenAddr, httpConfig.CertFile, httpConfig.KeyFile, r)
		} else {
			rr.logger.Println("TLS OFF", listenAddr)
			err = http.ListenAndServe(listenAddr, r)
		}
		if err != nil {
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
