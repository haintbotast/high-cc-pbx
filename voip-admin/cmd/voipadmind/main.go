package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"

	"github.com/yourusername/high-cc-pbx/voip-admin/internal/api"
	"github.com/yourusername/high-cc-pbx/voip-admin/internal/cache"
	"github.com/yourusername/high-cc-pbx/voip-admin/internal/database"
	"github.com/yourusername/high-cc-pbx/voip-admin/internal/middleware"
	"github.com/yourusername/high-cc-pbx/voip-admin/internal/workers"
)

const version = "1.0.0"

// Config represents the application configuration
type Config struct {
	Server struct {
		Host         string        `yaml:"host"`
		Port         int           `yaml:"port"`
		ReadTimeout  time.Duration `yaml:"read_timeout"`
		WriteTimeout time.Duration `yaml:"write_timeout"`
		IdleTimeout  time.Duration `yaml:"idle_timeout"`
	} `yaml:"server"`

	Database struct {
		Host            string        `yaml:"host"`
		Port            int           `yaml:"port"`
		User            string        `yaml:"user"`
		Password        string        `yaml:"password"`
		DBName          string        `yaml:"dbname"`
		SSLMode         string        `yaml:"sslmode"`
		MaxOpenConns    int           `yaml:"max_open_conns"`
		MaxIdleConns    int           `yaml:"max_idle_conns"`
		ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
		SearchPath      string        `yaml:"search_path"`
	} `yaml:"database"`

	Cache struct {
		Enabled         bool          `yaml:"enabled"`
		MaxEntries      int           `yaml:"max_entries"`
		CleanupInterval time.Duration `yaml:"cleanup_interval"`
	} `yaml:"cache"`

	CDR struct {
		BatchSize          int           `yaml:"batch_size"`
		ProcessingInterval time.Duration `yaml:"processing_interval"`
		CleanupInterval    time.Duration `yaml:"cleanup_interval"`
		RetentionDays      int           `yaml:"retention_days"`
	} `yaml:"cdr"`

	Auth struct {
		FreeSwitchUser     string   `yaml:"freeswitch_user"`
		FreeSwitchPassword string   `yaml:"freeswitch_password"`
		APIKeys            []string `yaml:"api_keys"`
	} `yaml:"auth"`

	CORS struct {
		Enabled          bool     `yaml:"enabled"`
		AllowedOrigins   []string `yaml:"allowed_origins"`
		AllowedMethods   []string `yaml:"allowed_methods"`
		AllowedHeaders   []string `yaml:"allowed_headers"`
		AllowCredentials bool     `yaml:"allow_credentials"`
	} `yaml:"cors"`
}

// Application holds the application state
type Application struct {
	Config       *Config
	DB           *database.DB
	Cache        *cache.Manager
	Router       *mux.Router
	CDRProcessor *workers.CDRProcessor
	CDRCleanup   *workers.CleanupWorker
}

func main() {
	// Parse command-line flags
	configFile := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	log.Printf("VoIP Admin Service v%s", version)
	log.Printf("Loading configuration from: %s", *configFile)

	// Load configuration
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	log.Println("Connecting to database...")
	db, err := database.New(&database.Config{
		Host:            config.Database.Host,
		Port:            config.Database.Port,
		User:            config.Database.User,
		Password:        config.Database.Password,
		DBName:          config.Database.DBName,
		SSLMode:         config.Database.SSLMode,
		MaxOpenConns:    config.Database.MaxOpenConns,
		MaxIdleConns:    config.Database.MaxIdleConns,
		ConnMaxLifetime: config.Database.ConnMaxLifetime,
		SearchPath:      config.Database.SearchPath,
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Database connected")

	// Initialize cache
	log.Println("Initializing cache...")
	cacheManager := cache.NewManager(&cache.Config{
		MaxEntries:      config.Cache.MaxEntries,
		CleanupInterval: config.Cache.CleanupInterval,
	})

	// Initialize CDR processor
	log.Println("Initializing CDR processor...")
	cdrProcessor := workers.NewCDRProcessor(db, &workers.CDRProcessorConfig{
		BatchSize:          config.CDR.BatchSize,
		ProcessingInterval: config.CDR.ProcessingInterval,
	})

	// Initialize CDR cleanup worker
	cdrCleanup := workers.NewCleanupWorker(db, config.CDR.CleanupInterval, config.CDR.RetentionDays)

	// Create application
	app := &Application{
		Config:       config,
		DB:           db,
		Cache:        cacheManager,
		Router:       mux.NewRouter(),
		CDRProcessor: cdrProcessor,
		CDRCleanup:   cdrCleanup,
	}

	// Setup routes
	log.Println("Setting up routes...")
	if err := app.setupRoutes(); err != nil {
		log.Fatalf("Failed to setup routes: %v", err)
	}

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      app.Router,
		ReadTimeout:  config.Server.ReadTimeout,
		WriteTimeout: config.Server.WriteTimeout,
		IdleTimeout:  config.Server.IdleTimeout,
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background workers
	log.Println("Starting background workers...")
	go cacheManager.Start(ctx)
	go cdrProcessor.Start(ctx)
	go cdrCleanup.Start(ctx)

	// Start HTTP server
	go func() {
		log.Printf("VoIP Admin Service listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down gracefully...")

	// Cancel context to stop background workers
	cancel()

	// Wait for workers to stop
	cacheManager.Stop()
	cdrProcessor.Stop()
	cdrCleanup.Stop()

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited cleanly")
}

// loadConfig loads configuration from YAML file
func loadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Set defaults
	if config.Server.ReadTimeout == 0 {
		config.Server.ReadTimeout = 10 * time.Second
	}
	if config.Server.WriteTimeout == 0 {
		config.Server.WriteTimeout = 10 * time.Second
	}
	if config.Server.IdleTimeout == 0 {
		config.Server.IdleTimeout = 60 * time.Second
	}
	if config.Database.MaxOpenConns == 0 {
		config.Database.MaxOpenConns = 50
	}
	if config.Database.MaxIdleConns == 0 {
		config.Database.MaxIdleConns = 10
	}
	if config.Database.ConnMaxLifetime == 0 {
		config.Database.ConnMaxLifetime = 5 * time.Minute
	}
	if config.Cache.MaxEntries == 0 {
		config.Cache.MaxEntries = 10000
	}
	if config.Cache.CleanupInterval == 0 {
		config.Cache.CleanupInterval = 60 * time.Second
	}
	if config.CDR.BatchSize == 0 {
		config.CDR.BatchSize = 100
	}
	if config.CDR.ProcessingInterval == 0 {
		config.CDR.ProcessingInterval = 5 * time.Second
	}
	if config.CDR.CleanupInterval == 0 {
		config.CDR.CleanupInterval = 24 * time.Hour
	}
	if config.CDR.RetentionDays == 0 {
		config.CDR.RetentionDays = 7
	}

	return &config, nil
}

// setupRoutes configures HTTP routes
func (app *Application) setupRoutes() error {
	// Initialize API handlers
	healthHandler := api.NewHealthHandler(app.DB, app.Cache, version)
	extensionHandler := api.NewExtensionHandler(app.DB)
	cdrHandler := api.NewCDRHandler(app.DB)

	freeSwitchHandler, err := api.NewFreeSwitchHandler(app.DB, app.Cache)
	if err != nil {
		return fmt.Errorf("create freeswitch handler: %w", err)
	}

	// Auth middleware config
	authConfig := &middleware.AuthConfig{
		FreeSwitchUser:     app.Config.Auth.FreeSwitchUser,
		FreeSwitchPassword: app.Config.Auth.FreeSwitchPassword,
		APIKeys:            app.Config.Auth.APIKeys,
	}

	// Apply global middleware
	app.Router.Use(middleware.Recovery)
	app.Router.Use(middleware.Logging)

	// CORS middleware (optional)
	if app.Config.CORS.Enabled {
		corsConfig := &middleware.CORSConfig{
			AllowedOrigins:   app.Config.CORS.AllowedOrigins,
			AllowedMethods:   app.Config.CORS.AllowedMethods,
			AllowedHeaders:   app.Config.CORS.AllowedHeaders,
			AllowCredentials: app.Config.CORS.AllowCredentials,
		}
		app.Router.Use(middleware.CORS(corsConfig))
	}

	// Public routes (no auth required)
	app.Router.HandleFunc("/health", healthHandler.Check).Methods("GET")
	app.Router.HandleFunc("/health/stats", healthHandler.Stats).Methods("GET")

	// FreeSWITCH XML_CURL endpoints (Basic Auth)
	fsRouter := app.Router.PathPrefix("/freeswitch").Subrouter()
	fsRouter.Use(middleware.BasicAuth(authConfig))
	fsRouter.HandleFunc("/directory", freeSwitchHandler.Directory).Methods("POST")
	fsRouter.HandleFunc("/dialplan", freeSwitchHandler.Dialplan).Methods("POST")
	fsRouter.HandleFunc("/configuration", freeSwitchHandler.Configuration).Methods("POST")

	// CDR ingest endpoint (Basic Auth - from FreeSWITCH)
	app.Router.Handle("/api/v1/cdr", middleware.BasicAuth(authConfig)(http.HandlerFunc(cdrHandler.Ingest))).Methods("POST")

	// Admin API endpoints (API Key Auth)
	apiRouter := app.Router.PathPrefix("/api/v1").Subrouter()
	apiRouter.Use(middleware.APIKeyAuth(authConfig))

	// CDR API
	apiRouter.HandleFunc("/cdr", cdrHandler.List).Methods("GET")
	apiRouter.HandleFunc("/cdr/{uuid}", cdrHandler.Get).Methods("GET")
	apiRouter.HandleFunc("/cdr/stats", cdrHandler.Stats).Methods("GET")

	// Extension API
	apiRouter.HandleFunc("/extensions", extensionHandler.List).Methods("GET")
	apiRouter.HandleFunc("/extensions", extensionHandler.Create).Methods("POST")
	apiRouter.HandleFunc("/extensions/{id}", extensionHandler.Get).Methods("GET")
	apiRouter.HandleFunc("/extensions/{id}", extensionHandler.Update).Methods("PUT")
	apiRouter.HandleFunc("/extensions/{id}", extensionHandler.Delete).Methods("DELETE")
	apiRouter.HandleFunc("/extensions/{id}/password", extensionHandler.UpdatePassword).Methods("POST")

	return nil
}
