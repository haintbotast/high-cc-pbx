package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/yourusername/high-cc-pbx/voip-admin/internal/database"
	"github.com/yourusername/high-cc-pbx/voip-admin/internal/models"
)

// CDRHandler handles CDR-related HTTP requests
type CDRHandler struct {
	db *database.DB
}

// NewCDRHandler creates a new CDR handler
func NewCDRHandler(db *database.DB) *CDRHandler {
	return &CDRHandler{
		db: db,
	}
}

// Ingest handles POST /api/v1/cdr (from FreeSWITCH)
func (h *CDRHandler) Ingest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Read XML body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Failed to read request body", err)
		return
	}

	if len(body) == 0 {
		respondError(w, http.StatusBadRequest, "Empty request body", nil)
		return
	}

	// Extract UUID from query parameter or parse XML
	uuid := r.URL.Query().Get("uuid")
	if uuid == "" {
		// Try to extract from XML (simple approach)
		// In production, should parse XML properly
		respondError(w, http.StatusBadRequest, "UUID is required", nil)
		return
	}

	// Insert into queue for async processing
	if err := h.db.InsertCDRQueue(ctx, uuid, string(body)); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to queue CDR", err)
		return
	}

	// Return immediately (async processing)
	w.WriteHeader(http.StatusAccepted)
}

// List handles GET /api/v1/cdr
func (h *CDRHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	req := &models.CDRListRequest{
		Page:    1,
		PerPage: 50,
	}

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			req.Page = p
		}
	}

	if perPageStr := r.URL.Query().Get("per_page"); perPageStr != "" {
		if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 1000 {
			req.PerPage = pp
		}
	}

	if startDateStr := r.URL.Query().Get("start_date"); startDateStr != "" {
		if t, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			req.StartDate = &t
		}
	}

	if endDateStr := r.URL.Query().Get("end_date"); endDateStr != "" {
		if t, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			req.EndDate = &t
		}
	}

	if callerID := r.URL.Query().Get("caller_id"); callerID != "" {
		req.CallerIDNumber = &callerID
	}

	if destNumber := r.URL.Query().Get("destination_number"); destNumber != "" {
		req.DestNumber = &destNumber
	}

	if direction := r.URL.Query().Get("direction"); direction != "" {
		req.Direction = &direction
	}

	if hangupCause := r.URL.Query().Get("hangup_cause"); hangupCause != "" {
		req.HangupCause = &hangupCause
	}

	if queueIDStr := r.URL.Query().Get("queue_id"); queueIDStr != "" {
		if id, err := strconv.ParseInt(queueIDStr, 10, 64); err == nil {
			req.QueueID = &id
		}
	}

	if minDurStr := r.URL.Query().Get("min_duration"); minDurStr != "" {
		if dur, err := strconv.Atoi(minDurStr); err == nil {
			req.MinDuration = &dur
		}
	}

	// Query database
	result, err := h.db.ListCDRs(ctx, req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list CDRs", err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// Get handles GET /api/v1/cdr/{uuid}
func (h *CDRHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uuid := r.URL.Query().Get("uuid")

	if uuid == "" {
		respondError(w, http.StatusBadRequest, "UUID is required", nil)
		return
	}

	cdr, err := h.db.GetCDRByUUID(ctx, uuid)
	if err != nil {
		respondError(w, http.StatusNotFound, "CDR not found", err)
		return
	}

	respondJSON(w, http.StatusOK, cdr)
}

// Stats handles GET /api/v1/cdr/stats
func (h *CDRHandler) Stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse date range (default to last 24 hours)
	endDate := time.Now()
	startDate := endDate.Add(-24 * time.Hour)

	if startDateStr := r.URL.Query().Get("start_date"); startDateStr != "" {
		if t, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			startDate = t
		}
	}

	if endDateStr := r.URL.Query().Get("end_date"); endDateStr != "" {
		if t, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			endDate = t
		}
	}

	stats, err := h.db.GetCDRStats(ctx, startDate, endDate)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get CDR stats", err)
		return
	}

	respondJSON(w, http.StatusOK, stats)
}
