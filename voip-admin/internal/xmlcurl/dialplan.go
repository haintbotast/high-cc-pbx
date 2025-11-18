package xmlcurl

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"regexp"

	"github.com/yourusername/high-cc-pbx/voip-admin/internal/database"
)

// DialplanHandler handles FreeSWITCH dialplan XML_CURL requests
type DialplanHandler struct {
	db        *database.DB
	templates map[string]*template.Template
}

// DialplanRequest represents a FreeSWITCH dialplan request
type DialplanRequest struct {
	Section         string // "dialplan"
	Context         string // "default", "public", etc.
	CallerIDNumber  string // Caller's number
	CallerIDName    string // Caller's name
	DestinationNumber string // Dialed number
	Domain          string // SIP domain
	NetworkAddr     string // Source IP
	ChannelName     string // Channel name
	UUID            string // Call UUID
}

// NewDialplanHandler creates a new dialplan handler
func NewDialplanHandler(db *database.DB) (*DialplanHandler, error) {
	templates := make(map[string]*template.Template)

	// Parse templates
	for name, content := range dialplanTemplates {
		tmpl, err := template.New(name).Parse(content)
		if err != nil {
			return nil, fmt.Errorf("parse dialplan template %s: %w", name, err)
		}
		templates[name] = tmpl
	}

	return &DialplanHandler{
		db:        db,
		templates: templates,
	}, nil
}

// Handle processes dialplan requests from FreeSWITCH
func (h *DialplanHandler) Handle(ctx context.Context, req *DialplanRequest) (string, error) {
	log.Printf("[Dialplan] Request: context=%s, caller=%s, destination=%s, domain=%s",
		req.Context, req.CallerIDNumber, req.DestinationNumber, req.Domain)

	// Route based on destination number pattern
	switch {
	case req.DestinationNumber == "":
		log.Printf("[Dialplan] Empty destination number")
		return h.renderNotFound(), nil

	case isExtension(req.DestinationNumber):
		// Local extension (4 digits: 1000-9999)
		return h.handleExtensionCall(ctx, req)

	case isQueue(req.DestinationNumber):
		// Queue (starts with 8: 8000-8999)
		return h.handleQueueCall(ctx, req)

	case isIVR(req.DestinationNumber):
		// IVR menu (starts with 9: 9000-9999)
		return h.handleIVRCall(ctx, req)

	case isConference(req.DestinationNumber):
		// Conference (starts with 3: 3000-3999)
		return h.handleConferenceCall(ctx, req)

	case isVoicemail(req.DestinationNumber):
		// Voicemail (*97, *98)
		return h.handleVoicemailCall(ctx, req)

	case isFeatureCode(req.DestinationNumber):
		// Feature codes (*xx)
		return h.handleFeatureCode(ctx, req)

	case isOutbound(req.DestinationNumber):
		// Outbound call (10-11 digits, or starts with +)
		return h.handleOutboundCall(ctx, req)

	default:
		log.Printf("[Dialplan] No matching pattern for destination: %s", req.DestinationNumber)
		return h.renderNotFound(), nil
	}
}

// handleExtensionCall handles calls to local extensions
func (h *DialplanHandler) handleExtensionCall(ctx context.Context, req *DialplanRequest) (string, error) {
	// Verify extension exists in database
	ext, err := h.db.GetExtension(ctx, req.DestinationNumber, req.Domain)
	if err != nil {
		log.Printf("[Dialplan] Extension not found: %s@%s", req.DestinationNumber, req.Domain)
		return h.renderNotFound(), nil
	}

	if !ext.Active {
		log.Printf("[Dialplan] Extension inactive: %s@%s", req.DestinationNumber, req.Domain)
		return h.renderNotFound(), nil
	}

	// Check extension type
	if ext.Type != "user" {
		log.Printf("[Dialplan] Cannot directly call non-user extension: %s@%s (type=%s)",
			req.DestinationNumber, req.Domain, ext.Type)
		return h.renderNotFound(), nil
	}

	data := struct {
		Extension       string
		Domain          string
		CallerIDNumber  string
		CallerIDName    string
		CallTimeout     int
		MaxConcurrent   int
	}{
		Extension:       ext.Extension,
		Domain:          req.Domain,
		CallerIDNumber:  req.CallerIDNumber,
		CallerIDName:    req.CallerIDName,
		CallTimeout:     ext.CallTimeout,
		MaxConcurrent:   ext.MaxConcurrent,
	}

	return h.renderTemplate("extension", data)
}

// handleQueueCall handles calls to queues
func (h *DialplanHandler) handleQueueCall(ctx context.Context, req *DialplanRequest) (string, error) {
	// Verify queue exists
	queue, err := h.db.GetQueueByExtension(ctx, req.DestinationNumber, req.Domain)
	if err != nil {
		log.Printf("[Dialplan] Queue not found: %s@%s", req.DestinationNumber, req.Domain)
		return h.renderNotFound(), nil
	}

	if !queue.Active {
		log.Printf("[Dialplan] Queue inactive: %s@%s", req.DestinationNumber, req.Domain)
		return h.renderNotFound(), nil
	}

	data := struct {
		QueueName      string
		Extension      string
		Domain         string
		CallerIDNumber string
		CallerIDName   string
		MaxWaitTime    int
		Moh            string
	}{
		QueueName:      queue.Name,
		Extension:      queue.Extension,
		Domain:         req.Domain,
		CallerIDNumber: req.CallerIDNumber,
		CallerIDName:   req.CallerIDName,
		MaxWaitTime:    queue.MaxWaitTime,
		Moh:            queue.Moh,
	}

	return h.renderTemplate("queue", data)
}

// handleIVRCall handles IVR menu calls
func (h *DialplanHandler) handleIVRCall(ctx context.Context, req *DialplanRequest) (string, error) {
	// TODO: Implement IVR logic when ivr table is ready
	log.Printf("[Dialplan] IVR not yet implemented: %s", req.DestinationNumber)
	return h.renderNotFound(), nil
}

// handleConferenceCall handles conference calls
func (h *DialplanHandler) handleConferenceCall(ctx context.Context, req *DialplanRequest) (string, error) {
	// Basic conference implementation
	data := struct {
		ConferenceNumber string
		Domain           string
		CallerIDNumber   string
		CallerIDName     string
	}{
		ConferenceNumber: req.DestinationNumber,
		Domain:           req.Domain,
		CallerIDNumber:   req.CallerIDNumber,
		CallerIDName:     req.CallerIDName,
	}

	return h.renderTemplate("conference", data)
}

// handleVoicemailCall handles voicemail access
func (h *DialplanHandler) handleVoicemailCall(ctx context.Context, req *DialplanRequest) (string, error) {
	data := struct {
		Domain         string
		CallerIDNumber string
		Extension      string
	}{
		Domain:         req.Domain,
		CallerIDNumber: req.CallerIDNumber,
		Extension:      req.CallerIDNumber, // Default to caller's own voicemail
	}

	return h.renderTemplate("voicemail", data)
}

// handleFeatureCode handles feature codes like call parking, pickup, etc.
func (h *DialplanHandler) handleFeatureCode(ctx context.Context, req *DialplanRequest) (string, error) {
	// Feature codes can be implemented here
	log.Printf("[Dialplan] Feature code not implemented: %s", req.DestinationNumber)
	return h.renderNotFound(), nil
}

// handleOutboundCall handles outbound PSTN calls
func (h *DialplanHandler) handleOutboundCall(ctx context.Context, req *DialplanRequest) (string, error) {
	// TODO: Implement trunk selection and outbound routing
	log.Printf("[Dialplan] Outbound call routing not yet implemented: %s", req.DestinationNumber)
	return h.renderNotFound(), nil
}

// Pattern matching functions
func isExtension(number string) bool {
	match, _ := regexp.MatchString(`^[1-79]\d{3}$`, number)
	return match
}

func isQueue(number string) bool {
	match, _ := regexp.MatchString(`^8\d{3}$`, number)
	return match
}

func isIVR(number string) bool {
	match, _ := regexp.MatchString(`^9\d{3}$`, number)
	return match
}

func isConference(number string) bool {
	match, _ := regexp.MatchString(`^3\d{3}$`, number)
	return match
}

func isVoicemail(number string) bool {
	return number == "*97" || number == "*98"
}

func isFeatureCode(number string) bool {
	match, _ := regexp.MatchString(`^\*\d{2,3}$`, number)
	return match
}

func isOutbound(number string) bool {
	// 10-11 digits or starts with +
	match, _ := regexp.MatchString(`^(\+?\d{10,11}|\+\d{8,15})$`, number)
	return match
}

// renderTemplate renders a dialplan template
func (h *DialplanHandler) renderTemplate(name string, data interface{}) (string, error) {
	tmpl, ok := h.templates[name]
	if !ok {
		return "", fmt.Errorf("template not found: %s", name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// renderNotFound renders a "not found" XML response
func (h *DialplanHandler) renderNotFound() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<document type="freeswitch/xml">
  <section name="result">
    <result status="not found"/>
  </section>
</document>`
}

// dialplanTemplates contains XML templates for different call types
var dialplanTemplates = map[string]string{
	"extension": `<?xml version="1.0" encoding="UTF-8"?>
<document type="freeswitch/xml">
  <section name="dialplan" description="Extension Dialplan">
    <context name="default">
      <extension name="local_extension">
        <condition field="destination_number" expression="^{{.Extension}}$">
          <action application="set" data="call_timeout={{.CallTimeout}}"/>
          <action application="set" data="hangup_after_bridge=true"/>
          <action application="set" data="continue_on_fail=true"/>
          <action application="set" data="called_party_callgroup=${user_data({{.Extension}}@{{.Domain}} var callgroup)}"/>
          <action application="export" data="dialed_extension={{.Extension}}"/>

          <!-- Pre-answer for queue calls -->
          <action application="ring_ready" data=""/>

          <!-- Bridge to extension with recording -->
          <action application="set" data="RECORD_TITLE={{.CallerIDNumber}} to {{.Extension}}"/>
          <action application="set" data="RECORD_COPYRIGHT=High CC PBX"/>
          <action application="set" data="RECORD_ARTIST={{.CallerIDNumber}}"/>
          <action application="set" data="RECORD_DATE=${strftime(%Y-%m-%d %H:%M:%S)}"/>

          <action application="bridge" data="user/{{.Extension}}@{{.Domain}}"/>

          <!-- Voicemail on no answer or busy -->
          <action application="answer" data=""/>
          <action application="sleep" data="1000"/>
          <action application="voicemail" data="default {{.Domain}} {{.Extension}}"/>
        </condition>
      </extension>
    </context>
  </section>
</document>`,

	"queue": `<?xml version="1.0" encoding="UTF-8"?>
<document type="freeswitch/xml">
  <section name="dialplan" description="Queue Dialplan">
    <context name="default">
      <extension name="queue_call">
        <condition field="destination_number" expression="^{{.Extension}}$">
          <action application="answer" data=""/>
          <action application="set" data="hangup_after_bridge=true"/>
          <action application="set" data="continue_on_fail=NORMAL_TEMPORARY_FAILURE,USER_BUSY,NO_ANSWER,TIMEOUT,NO_USER_RESPONSE"/>

          <!-- Set queue variables -->
          <action application="set" data="queue_name={{.QueueName}}"/>
          <action application="set" data="max_wait_time={{.MaxWaitTime}}"/>

          <!-- Enter queue with music on hold -->
          <action application="callcenter" data="{{.QueueName}}@{{.Domain}}"/>

          <!-- Fallback if queue fails -->
          <action application="playback" data="ivr/ivr-call_cannot_be_completed_as_dialed.wav"/>
          <action application="hangup" data=""/>
        </condition>
      </extension>
    </context>
  </section>
</document>`,

	"conference": `<?xml version="1.0" encoding="UTF-8"?>
<document type="freeswitch/xml">
  <section name="dialplan" description="Conference Dialplan">
    <context name="default">
      <extension name="conference_call">
        <condition field="destination_number" expression="^{{.ConferenceNumber}}$">
          <action application="answer" data=""/>
          <action application="set" data="conference_name={{.ConferenceNumber}}@{{.Domain}}"/>

          <!-- Join conference -->
          <action application="conference" data="{{.ConferenceNumber}}@default"/>
        </condition>
      </extension>
    </context>
  </section>
</document>`,

	"voicemail": `<?xml version="1.0" encoding="UTF-8"?>
<document type="freeswitch/xml">
  <section name="dialplan" description="Voicemail Access">
    <context name="default">
      <extension name="voicemail_check">
        <condition field="destination_number" expression="^(\*97|\*98)$">
          <action application="answer" data=""/>
          <action application="sleep" data="1000"/>
          <action application="voicemail" data="check default {{.Domain}} {{.Extension}}"/>
        </condition>
      </extension>
    </context>
  </section>
</document>`,
}
