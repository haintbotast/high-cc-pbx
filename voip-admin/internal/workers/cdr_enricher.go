package workers

import (
	"context"
	"fmt"
	"log"
	"regexp"

	"github.com/yourusername/high-cc-pbx/voip-admin/internal/database"
	"github.com/yourusername/high-cc-pbx/voip-admin/internal/models"
)

// CDREnricher enriches CDR records with business logic
type CDREnricher struct {
	db *database.DB
}

// NewCDREnricher creates a new CDR enricher
func NewCDREnricher(db *database.DB) *CDREnricher {
	return &CDREnricher{
		db: db,
	}
}

// Enrich adds business logic and additional data to a CDR
func (e *CDREnricher) Enrich(ctx context.Context, cdr *models.CDR) error {
	// 1. Determine call type if not already set
	if cdr.CallType == "" {
		cdr.CallType = e.determineCallType(cdr.DestinationNumber)
	}

	// 2. Map queue if call type is queue
	if cdr.CallType == "queue" {
		if err := e.enrichQueueInfo(ctx, cdr); err != nil {
			log.Printf("[CDREnricher] Queue enrichment failed for %s: %v", cdr.UUID, err)
			// Don't fail the entire enrichment if queue lookup fails
		}
	}

	// 3. Validate and normalize direction
	if cdr.Direction == "" {
		cdr.Direction = e.determineDirection(cdr.CallerIDNumber, cdr.DestinationNumber)
	}

	// 4. Enrich with extension information
	if err := e.enrichExtensionInfo(ctx, cdr); err != nil {
		log.Printf("[CDREnricher] Extension enrichment failed for %s: %v", cdr.UUID, err)
		// Don't fail if extension lookup fails
	}

	return nil
}

// determineCallType determines the call type based on destination number pattern
func (e *CDREnricher) determineCallType(destNumber string) string {
	switch {
	case isQueuePattern(destNumber):
		return "queue"
	case isIVRPattern(destNumber):
		return "ivr"
	case isConferencePattern(destNumber):
		return "conference"
	case isExtensionPattern(destNumber):
		return "direct"
	default:
		return "other"
	}
}

// determineDirection determines call direction
func (e *CDREnricher) determineDirection(callerNumber, destNumber string) string {
	// If both are extensions (4 digits), it's internal
	if isExtensionPattern(callerNumber) && isExtensionPattern(destNumber) {
		return "internal"
	}

	// If destination is 10+ digits or starts with +, it's outbound
	if len(destNumber) >= 10 || regexp.MustCompile(`^\+`).MatchString(destNumber) {
		return "outbound"
	}

	// If caller is external (not 4 digits) and dest is extension, it's inbound
	if !isExtensionPattern(callerNumber) && isExtensionPattern(destNumber) {
		return "inbound"
	}

	// Default to internal
	return "internal"
}

// enrichQueueInfo enriches queue-specific information
func (e *CDREnricher) enrichQueueInfo(ctx context.Context, cdr *models.CDR) error {
	// Try to find queue by extension number
	queue, err := e.db.GetQueueByExtension(ctx, cdr.DestinationNumber, cdr.Domain)
	if err != nil {
		return fmt.Errorf("get queue by extension: %w", err)
	}

	// Set queue ID
	cdr.QueueID = &queue.ID

	log.Printf("[CDREnricher] Mapped queue %s to ID %d for CDR %s",
		queue.Name, queue.ID, cdr.UUID)

	return nil
}

// enrichExtensionInfo enriches extension information
func (e *CDREnricher) enrichExtensionInfo(ctx context.Context, cdr *models.CDR) error {
	// Try to get destination extension info
	if isExtensionPattern(cdr.DestinationNumber) && cdr.Domain != "" {
		ext, err := e.db.GetExtension(ctx, cdr.DestinationNumber, cdr.Domain)
		if err == nil {
			// Store additional extension info if needed
			log.Printf("[CDREnricher] Found destination extension %s (id=%d) for CDR %s",
				ext.Extension, ext.ID, cdr.UUID)
		}
	}

	return nil
}

// Pattern matching helpers
func isQueuePattern(number string) bool {
	match, _ := regexp.MatchString(`^8\d{3}$`, number)
	return match
}

func isIVRPattern(number string) bool {
	match, _ := regexp.MatchString(`^9\d{3}$`, number)
	return match
}

func isConferencePattern(number string) bool {
	match, _ := regexp.MatchString(`^3\d{3}$`, number)
	return match
}

func isExtensionPattern(number string) bool {
	match, _ := regexp.MatchString(`^[1-79]\d{3}$`, number)
	return match
}
