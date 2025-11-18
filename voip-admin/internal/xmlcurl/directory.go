package xmlcurl

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"time"

	"github.com/yourusername/high-cc-pbx/voip-admin/internal/database"
	"github.com/yourusername/high-cc-pbx/voip-admin/internal/models"
)

// DirectoryHandler handles FreeSWITCH directory XML_CURL requests
type DirectoryHandler struct {
	db       *database.DB
	cache    Cache
	template *template.Template
}

// Cache interface for directory caching
type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Delete(key string)
}

// DirectoryRequest represents a FreeSWITCH directory request
type DirectoryRequest struct {
	Section   string // "directory"
	TagName   string // "domain"
	KeyName   string // "name"
	KeyValue  string // domain name
	User      string // user/extension
	Domain    string // domain
	Action    string // "sip_auth", "message-count", etc.
	SIPAuth   string // "true" for auth requests
	Purpose   string // "publish", "subscribe", etc.
}

// NewDirectoryHandler creates a new directory handler
func NewDirectoryHandler(db *database.DB, cache Cache) (*DirectoryHandler, error) {
	// Parse directory XML template
	tmpl, err := template.New("directory").Parse(directoryTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse directory template: %w", err)
	}

	return &DirectoryHandler{
		db:       db,
		cache:    cache,
		template: tmpl,
	}, nil
}

// Handle processes directory requests from FreeSWITCH
func (h *DirectoryHandler) Handle(ctx context.Context, req *DirectoryRequest) (string, error) {
	// Log the request for debugging
	log.Printf("[Directory] Request: user=%s, domain=%s, action=%s", req.User, req.Domain, req.Action)

	// Validate required fields
	if req.User == "" || req.Domain == "" {
		return h.renderNotFound(), nil
	}

	// Try cache first (60s TTL)
	cacheKey := fmt.Sprintf("dir:%s@%s", req.User, req.Domain)
	if cached, ok := h.cache.Get(cacheKey); ok {
		if ext, ok := cached.(*models.Extension); ok {
			log.Printf("[Directory] Cache hit for %s@%s", req.User, req.Domain)
			return h.renderDirectory(ext)
		}
	}

	// Query database
	ext, err := h.db.GetExtension(ctx, req.User, req.Domain)
	if err != nil {
		log.Printf("[Directory] Extension not found: %s@%s - %v", req.User, req.Domain, err)
		return h.renderNotFound(), nil
	}

	// Check if extension is active
	if !ext.Active {
		log.Printf("[Directory] Extension inactive: %s@%s", req.User, req.Domain)
		return h.renderNotFound(), nil
	}

	// Only allow 'user' type extensions to authenticate
	if ext.Type != "user" {
		log.Printf("[Directory] Invalid extension type for auth: %s@%s (type=%s)", req.User, req.Domain, ext.Type)
		return h.renderNotFound(), nil
	}

	// Cache the result (60s TTL)
	h.cache.Set(cacheKey, ext, 60*time.Second)

	log.Printf("[Directory] Found extension: %s@%s (id=%d)", req.User, req.Domain, ext.ID)

	return h.renderDirectory(ext)
}

// renderDirectory renders the directory XML response for an extension
func (h *DirectoryHandler) renderDirectory(ext *models.Extension) (string, error) {
	data := struct {
		Extension    string
		Domain       string
		HA1          string
		HA1B         string
		DisplayName  string
		VMPassword   string
		VMEmail      string
		MaxConcurrent int
		CallTimeout  int
	}{
		Extension:    ext.Extension,
		Domain:       ext.Domain,
		HA1:          ext.SIPHA1,
		HA1B:         ext.SIPHA1B,
		DisplayName:  ext.DisplayName,
		VMPassword:   ext.VMPassword,
		VMEmail:      ext.VMEmail,
		MaxConcurrent: ext.MaxConcurrent,
		CallTimeout:  ext.CallTimeout,
	}

	var buf bytes.Buffer
	if err := h.template.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// renderNotFound renders a "not found" XML response
func (h *DirectoryHandler) renderNotFound() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<document type="freeswitch/xml">
  <section name="result">
    <result status="not found"/>
  </section>
</document>`
}

// InvalidateCache invalidates the cache for a specific user
func (h *DirectoryHandler) InvalidateCache(user, domain string) {
	cacheKey := fmt.Sprintf("dir:%s@%s", user, domain)
	h.cache.Delete(cacheKey)
	log.Printf("[Directory] Cache invalidated for %s@%s", user, domain)
}

// directoryTemplate is the XML template for FreeSWITCH directory responses
// Uses MD5 digest authentication with HA1/HA1B hashes
const directoryTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<document type="freeswitch/xml">
  <section name="directory">
    <domain name="{{.Domain}}">
      <params>
        <param name="dial-string" value="{^^:sip_invite_domain=${dialed_domain}:presence_id=${dialed_user}@${dialed_domain}}${sofia_contact(*/${dialed_user}@${dialed_domain})},${verto_contact(${dialed_user}@${dialed_domain})}"/>
        <param name="jsonrpc-allowed-methods" value="verto"/>
      </params>

      <groups>
        <group name="default">
          <users>
            <user id="{{.Extension}}">
              <params>
                <!-- MD5 Digest Authentication (HA1 hashes calculated by database) -->
                <param name="a1-hash" value="{{.HA1}}"/>
                <param name="a1-hash-b" value="{{.HA1B}}"/>

                <!-- User Settings -->
                <param name="dial-string" value="{presence_id=${dialed_user}@${dialed_domain}}${sofia_contact(${dialed_user}@${dialed_domain})}"/>
                <param name="max-calls" value="{{.MaxConcurrent}}"/>

                <!-- Voicemail Settings -->
                {{if .VMPassword}}<param name="vm-password" value="{{.VMPassword}}"/>{{end}}
                {{if .VMEmail}}<param name="vm-email-all-messages" value="{{.VMEmail}}"/>{{end}}
              </params>

              <variables>
                <variable name="toll_allow" value="domestic,international"/>
                <variable name="accountcode" value="{{.Extension}}"/>
                <variable name="user_context" value="default"/>
                <variable name="effective_caller_id_name" value="{{.DisplayName}}"/>
                <variable name="effective_caller_id_number" value="{{.Extension}}"/>
                <variable name="outbound_caller_id_name" value="{{.DisplayName}}"/>
                <variable name="outbound_caller_id_number" value="{{.Extension}}"/>
                <variable name="callgroup" value="default"/>
                <variable name="call_timeout" value="{{.CallTimeout}}"/>
              </variables>
            </user>
          </users>
        </group>
      </groups>
    </domain>
  </section>
</document>`
