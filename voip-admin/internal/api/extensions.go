package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/yourusername/high-cc-pbx/voip-admin/internal/database"
	"github.com/yourusername/high-cc-pbx/voip-admin/internal/models"
)

// ExtensionHandler handles extension-related HTTP requests
type ExtensionHandler struct {
	db *database.DB
}

// NewExtensionHandler creates a new extension handler
func NewExtensionHandler(db *database.DB) *ExtensionHandler {
	return &ExtensionHandler{
		db: db,
	}
}

// List handles GET /api/v1/extensions
func (h *ExtensionHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	var domainID *int64
	if domainIDStr := r.URL.Query().Get("domain_id"); domainIDStr != "" {
		id, err := strconv.ParseInt(domainIDStr, 10, 64)
		if err == nil {
			domainID = &id
		}
	}

	var extType *string
	if typeStr := r.URL.Query().Get("type"); typeStr != "" {
		extType = &typeStr
	}

	var active *bool
	if activeStr := r.URL.Query().Get("active"); activeStr != "" {
		activeBool := activeStr == "true" || activeStr == "1"
		active = &activeBool
	}

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	perPage := 50
	if perPageStr := r.URL.Query().Get("per_page"); perPageStr != "" {
		if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 1000 {
			perPage = pp
		}
	}

	// Query database
	result, err := h.db.ListExtensions(ctx, domainID, extType, active, page, perPage)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list extensions", err)
		return
	}

	// Remove sensitive data before sending
	for _, ext := range result.Extensions {
		ext.SIPPassword = ""
		ext.SIPHA1 = ""
		ext.SIPHA1B = ""
	}

	respondJSON(w, http.StatusOK, result)
}

// Get handles GET /api/v1/extensions/{id}
func (h *ExtensionHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)

	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid extension ID", err)
		return
	}

	ext, err := h.db.GetExtensionByID(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Extension not found", err)
		return
	}

	// Remove sensitive data
	ext.SIPPassword = ""
	ext.SIPHA1 = ""
	ext.SIPHA1B = ""

	respondJSON(w, http.StatusOK, ext)
}

// Create handles POST /api/v1/extensions
func (h *ExtensionHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.ExtensionCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate request
	if err := validateExtensionCreateRequest(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Validation failed", err)
		return
	}

	// Create extension
	ext, err := h.db.CreateExtension(ctx, &req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create extension", err)
		return
	}

	// Remove sensitive data
	ext.SIPPassword = ""
	ext.SIPHA1 = ""
	ext.SIPHA1B = ""

	respondJSON(w, http.StatusCreated, ext)
}

// Update handles PUT /api/v1/extensions/{id}
func (h *ExtensionHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)

	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid extension ID", err)
		return
	}

	var req models.ExtensionUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate request
	if err := validateExtensionUpdateRequest(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Validation failed", err)
		return
	}

	// Update extension
	ext, err := h.db.UpdateExtension(ctx, id, &req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update extension", err)
		return
	}

	// Remove sensitive data
	ext.SIPPassword = ""
	ext.SIPHA1 = ""
	ext.SIPHA1B = ""

	respondJSON(w, http.StatusOK, ext)
}

// Delete handles DELETE /api/v1/extensions/{id}
func (h *ExtensionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)

	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid extension ID", err)
		return
	}

	if err := h.db.DeleteExtension(ctx, id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete extension", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdatePassword handles POST /api/v1/extensions/{id}/password
func (h *ExtensionHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)

	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid extension ID", err)
		return
	}

	var req models.ExtensionPasswordUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate passwords
	if req.NewPassword == "" || len(req.NewPassword) < 8 {
		respondError(w, http.StatusBadRequest, "New password must be at least 8 characters", nil)
		return
	}

	// TODO: Verify old password before updating
	// For now, just update with new password

	if err := h.db.UpdateExtensionPassword(ctx, id, req.NewPassword); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update password", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Password updated successfully"})
}

// Validation helpers
func validateExtensionCreateRequest(req *models.ExtensionCreateRequest) error {
	if req.DomainID == 0 {
		return errValidation("domain_id is required")
	}
	if req.Extension == "" {
		return errValidation("extension is required")
	}
	if len(req.Extension) < 3 || len(req.Extension) > 20 {
		return errValidation("extension must be 3-20 characters")
	}
	if req.Type == "" {
		return errValidation("type is required")
	}
	validTypes := map[string]bool{"user": true, "queue": true, "ivr": true, "conference": true}
	if !validTypes[req.Type] {
		return errValidation("type must be one of: user, queue, ivr, conference")
	}
	if req.DisplayName == "" {
		return errValidation("display_name is required")
	}
	if req.SIPPassword == "" || len(req.SIPPassword) < 8 {
		return errValidation("sip_password must be at least 8 characters")
	}
	if req.MaxConcurrent < 1 || req.MaxConcurrent > 100 {
		return errValidation("max_concurrent must be 1-100")
	}
	if req.CallTimeout < 10 || req.CallTimeout > 300 {
		return errValidation("call_timeout must be 10-300 seconds")
	}
	return nil
}

func validateExtensionUpdateRequest(req *models.ExtensionUpdateRequest) error {
	if req.DisplayName != nil && *req.DisplayName == "" {
		return errValidation("display_name cannot be empty")
	}
	if req.MaxConcurrent != nil && (*req.MaxConcurrent < 1 || *req.MaxConcurrent > 100) {
		return errValidation("max_concurrent must be 1-100")
	}
	if req.CallTimeout != nil && (*req.CallTimeout < 10 || *req.CallTimeout > 300) {
		return errValidation("call_timeout must be 10-300 seconds")
	}
	return nil
}
