package api

import (
	"log"
	"net/http"

	"github.com/yourusername/high-cc-pbx/voip-admin/internal/database"
	"github.com/yourusername/high-cc-pbx/voip-admin/internal/xmlcurl"
)

// FreeSwitchHandler handles FreeSWITCH XML_CURL requests
type FreeSwitchHandler struct {
	directoryHandler     *xmlcurl.DirectoryHandler
	dialplanHandler      *xmlcurl.DialplanHandler
	configurationHandler *xmlcurl.ConfigurationHandler
}

// NewFreeSwitchHandler creates a new FreeSWITCH handler
func NewFreeSwitchHandler(db *database.DB, cache xmlcurl.Cache) (*FreeSwitchHandler, error) {
	directoryHandler, err := xmlcurl.NewDirectoryHandler(db, cache)
	if err != nil {
		return nil, err
	}

	dialplanHandler, err := xmlcurl.NewDialplanHandler(db)
	if err != nil {
		return nil, err
	}

	configurationHandler := xmlcurl.NewConfigurationHandler(db)

	return &FreeSwitchHandler{
		directoryHandler:     directoryHandler,
		dialplanHandler:      dialplanHandler,
		configurationHandler: configurationHandler,
	}, nil
}

// Directory handles POST /freeswitch/directory
func (h *FreeSwitchHandler) Directory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request
	req, err := xmlcurl.ParseDirectoryRequest(r)
	if err != nil {
		log.Printf("[FreeSWITCH] Failed to parse directory request: %v", err)
		respondXML(w, http.StatusOK, notFoundXML())
		return
	}

	// Validate request
	if err := xmlcurl.ValidateDirectoryRequest(req); err != nil {
		log.Printf("[FreeSWITCH] Invalid directory request: %v", err)
		respondXML(w, http.StatusOK, notFoundXML())
		return
	}

	// Handle request
	xml, err := h.directoryHandler.Handle(ctx, req)
	if err != nil {
		log.Printf("[FreeSWITCH] Directory handler error: %v", err)
		respondXML(w, http.StatusOK, notFoundXML())
		return
	}

	respondXML(w, http.StatusOK, xml)
}

// Dialplan handles POST /freeswitch/dialplan
func (h *FreeSwitchHandler) Dialplan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request
	req, err := xmlcurl.ParseDialplanRequest(r)
	if err != nil {
		log.Printf("[FreeSWITCH] Failed to parse dialplan request: %v", err)
		respondXML(w, http.StatusOK, notFoundXML())
		return
	}

	// Validate request
	if err := xmlcurl.ValidateDialplanRequest(req); err != nil {
		log.Printf("[FreeSWITCH] Invalid dialplan request: %v", err)
		respondXML(w, http.StatusOK, notFoundXML())
		return
	}

	// Handle request
	xml, err := h.dialplanHandler.Handle(ctx, req)
	if err != nil {
		log.Printf("[FreeSWITCH] Dialplan handler error: %v", err)
		respondXML(w, http.StatusOK, notFoundXML())
		return
	}

	respondXML(w, http.StatusOK, xml)
}

// Configuration handles POST /freeswitch/configuration
func (h *FreeSwitchHandler) Configuration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request
	req, err := xmlcurl.ParseConfigurationRequest(r)
	if err != nil {
		log.Printf("[FreeSWITCH] Failed to parse configuration request: %v", err)
		respondXML(w, http.StatusOK, notFoundXML())
		return
	}

	// Validate request
	if err := xmlcurl.ValidateConfigurationRequest(req); err != nil {
		log.Printf("[FreeSWITCH] Invalid configuration request: %v", err)
		respondXML(w, http.StatusOK, notFoundXML())
		return
	}

	// Handle request
	xml, err := h.configurationHandler.Handle(ctx, req)
	if err != nil {
		log.Printf("[FreeSWITCH] Configuration handler error: %v", err)
		respondXML(w, http.StatusOK, notFoundXML())
		return
	}

	respondXML(w, http.StatusOK, xml)
}

// notFoundXML returns a "not found" XML response
func notFoundXML() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<document type="freeswitch/xml">
  <section name="result">
    <result status="not found"/>
  </section>
</document>`
}
