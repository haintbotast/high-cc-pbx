package api

import (
	"net/http"
	"time"

	"github.com/yourusername/high-cc-pbx/voip-admin/internal/cache"
	"github.com/yourusername/high-cc-pbx/voip-admin/internal/database"
	"github.com/yourusername/high-cc-pbx/voip-admin/internal/models"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	db      *database.DB
	cache   *cache.Manager
	version string
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *database.DB, cache *cache.Manager, version string) *HealthHandler {
	return &HealthHandler{
		db:      db,
		cache:   cache,
		version: version,
	}
}

// Check handles GET /health
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	response := &models.HealthResponse{
		Status:    "ok",
		Service:   "voip-admin",
		Version:   h.version,
		Timestamp: time.Now(),
		Database:  "ok",
		Cache:     "ok",
	}

	// Check database
	if err := h.db.Health(ctx); err != nil {
		response.Status = "degraded"
		response.Database = "error: " + err.Error()
	}

	// Check cache
	stats := h.cache.Stats()
	if stats.Size < 0 {
		response.Status = "degraded"
		response.Cache = "error"
	}

	statusCode := http.StatusOK
	if response.Status != "ok" {
		statusCode = http.StatusServiceUnavailable
	}

	respondJSON(w, statusCode, response)
}

// Stats handles GET /health/stats
func (h *HealthHandler) Stats(w http.ResponseWriter, r *http.Request) {
	dbStats := h.db.Stats()
	cacheStats := h.cache.Stats()

	stats := map[string]interface{}{
		"database": map[string]interface{}{
			"max_open_connections": dbStats.MaxOpenConnections,
			"open_connections":     dbStats.OpenConnections,
			"in_use":               dbStats.InUse,
			"idle":                 dbStats.Idle,
			"wait_count":           dbStats.WaitCount,
			"wait_duration":        dbStats.WaitDuration.String(),
			"max_idle_closed":      dbStats.MaxIdleClosed,
			"max_lifetime_closed":  dbStats.MaxLifetimeClosed,
		},
		"cache": cacheStats,
	}

	respondJSON(w, http.StatusOK, stats)
}
