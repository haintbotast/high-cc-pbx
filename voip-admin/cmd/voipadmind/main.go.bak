package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"
)

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
		TTL             time.Duration `yaml:"ttl"`
		CleanupInterval time.Duration `yaml:"cleanup_interval"`
		MaxEntries      int           `yaml:"max_entries"`
	} `yaml:"cache"`

	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
		Output string `yaml:"output"`
	} `yaml:"logging"`

	Health struct {
		Enabled bool   `yaml:"enabled"`
		Path    string `yaml:"path"`
	} `yaml:"health"`
}

// Application holds the application state
type Application struct {
	Config *Config
	Router *mux.Router
	// DB, Cache, etc. will be added here
}

func main() {
	// Load configuration
	config, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize application
	app := &Application{
		Config: config,
		Router: mux.NewRouter(),
	}

	// Setup routes
	app.setupRoutes()

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      app.Router,
		ReadTimeout:  config.Server.ReadTimeout,
		WriteTimeout: config.Server.WriteTimeout,
		IdleTimeout:  config.Server.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Printf("VoIP Admin Service starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
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

	return &config, nil
}

// setupRoutes configures HTTP routes
func (app *Application) setupRoutes() {
	// Health check
	if app.Config.Health.Enabled {
		app.Router.HandleFunc(app.Config.Health.Path, app.healthHandler).Methods("GET")
	}

	// FreeSWITCH XML_CURL endpoints
	fs := app.Router.PathPrefix("/freeswitch").Subrouter()
	fs.HandleFunc("/directory", app.xmlCurlDirectoryHandler).Methods("POST")
	fs.HandleFunc("/dialplan", app.xmlCurlDialplanHandler).Methods("POST")
	fs.HandleFunc("/configuration", app.xmlCurlConfigHandler).Methods("POST")

	// CDR endpoints
	app.Router.HandleFunc("/api/v1/cdr", app.cdrPostHandler).Methods("POST")
	app.Router.HandleFunc("/api/v1/cdr", app.cdrListHandler).Methods("GET")

	// Extension management
	app.Router.HandleFunc("/api/v1/extensions", app.extensionListHandler).Methods("GET")
	app.Router.HandleFunc("/api/v1/extensions", app.extensionCreateHandler).Methods("POST")
	app.Router.HandleFunc("/api/v1/extensions/{id}", app.extensionGetHandler).Methods("GET")
	app.Router.HandleFunc("/api/v1/extensions/{id}", app.extensionUpdateHandler).Methods("PUT")
	app.Router.HandleFunc("/api/v1/extensions/{id}", app.extensionDeleteHandler).Methods("DELETE")

	// TODO: Add more routes for queues, IVR, trunks, etc.
}

// Health check handler
func (app *Application) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","service":"voip-admin"}`))
}

// FreeSWITCH XML_CURL handlers (placeholders)
func (app *Application) xmlCurlDirectoryHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement directory lookup from database
	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<document type="freeswitch/xml">
  <section name="result">
    <result status="not found"/>
  </section>
</document>`))
}

func (app *Application) xmlCurlDialplanHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement dialplan generation from database
	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<document type="freeswitch/xml">
  <section name="result">
    <result status="not found"/>
  </section>
</document>`))
}

func (app *Application) xmlCurlConfigHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement configuration generation
	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<document type="freeswitch/xml">
  <section name="result">
    <result status="not found"/>
  </section>
</document>`))
}

// CDR handlers (placeholders)
func (app *Application) cdrPostHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement CDR ingestion
	w.WriteHeader(http.StatusAccepted)
}

func (app *Application) cdrListHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement CDR listing with pagination
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"cdrs":[]}`))
}

// Extension handlers (placeholders)
func (app *Application) extensionListHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement extension listing
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"extensions":[]}`))
}

func (app *Application) extensionCreateHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement extension creation
	w.WriteHeader(http.StatusCreated)
}

func (app *Application) extensionGetHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement extension retrieval
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"extension":{}}`))
}

func (app *Application) extensionUpdateHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement extension update
	w.WriteHeader(http.StatusOK)
}

func (app *Application) extensionDeleteHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement extension deletion
	w.WriteHeader(http.StatusNoContent)
}
