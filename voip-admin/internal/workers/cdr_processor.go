package workers

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yourusername/high-cc-pbx/voip-admin/internal/database"
)

// CDRProcessor processes CDRs from the queue asynchronously
type CDRProcessor struct {
	db              *database.DB
	batchSize       int
	processingInterval time.Duration
	enricher        *CDREnricher
	done            chan struct{}
}

// CDRProcessorConfig holds configuration for the CDR processor
type CDRProcessorConfig struct {
	BatchSize          int           // Number of CDRs to process per batch
	ProcessingInterval time.Duration // How often to process batches
}

// NewCDRProcessor creates a new CDR processor
func NewCDRProcessor(db *database.DB, cfg *CDRProcessorConfig) *CDRProcessor {
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 100
	}
	if cfg.ProcessingInterval == 0 {
		cfg.ProcessingInterval = 5 * time.Second
	}

	return &CDRProcessor{
		db:              db,
		batchSize:       cfg.BatchSize,
		processingInterval: cfg.ProcessingInterval,
		enricher:        NewCDREnricher(db),
		done:            make(chan struct{}),
	}
}

// Start begins processing CDRs in the background
func (p *CDRProcessor) Start(ctx context.Context) {
	log.Printf("[CDRProcessor] Starting with batch_size=%d, interval=%v",
		p.batchSize, p.processingInterval)

	ticker := time.NewTicker(p.processingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[CDRProcessor] Shutting down...")
			close(p.done)
			return

		case <-ticker.C:
			if err := p.processBatch(ctx); err != nil {
				log.Printf("[CDRProcessor] Error processing batch: %v", err)
			}
		}
	}
}

// processBatch processes a batch of pending CDRs
func (p *CDRProcessor) processBatch(ctx context.Context) error {
	// Fetch pending CDRs from queue
	queuedCDRs, err := p.db.GetPendingCDRs(ctx, p.batchSize)
	if err != nil {
		return fmt.Errorf("get pending cdrs: %w", err)
	}

	if len(queuedCDRs) == 0 {
		return nil // No CDRs to process
	}

	log.Printf("[CDRProcessor] Processing %d CDRs", len(queuedCDRs))

	successCount := 0
	failCount := 0

	for _, queuedCDR := range queuedCDRs {
		if err := p.processOne(ctx, queuedCDR.ID, queuedCDR.UUID, queuedCDR.XMLData); err != nil {
			log.Printf("[CDRProcessor] Failed to process CDR %s (id=%d): %v",
				queuedCDR.UUID, queuedCDR.ID, err)

			// Mark as failed
			if markErr := p.db.MarkCDRFailed(ctx, queuedCDR.ID, err.Error()); markErr != nil {
				log.Printf("[CDRProcessor] Failed to mark CDR as failed: %v", markErr)
			}
			failCount++
		} else {
			// Mark as processed
			if markErr := p.db.MarkCDRProcessed(ctx, queuedCDR.ID); markErr != nil {
				log.Printf("[CDRProcessor] Failed to mark CDR as processed: %v", markErr)
			}
			successCount++
		}
	}

	log.Printf("[CDRProcessor] Batch complete: success=%d, failed=%d", successCount, failCount)
	return nil
}

// processOne processes a single CDR
func (p *CDRProcessor) processOne(ctx context.Context, queueID int64, uuid, xmlData string) error {
	// Parse XML
	cdr, err := ParseCDRXML(xmlData)
	if err != nil {
		return fmt.Errorf("parse CDR XML: %w", err)
	}

	// Verify UUID matches
	if cdr.UUID != uuid {
		return fmt.Errorf("UUID mismatch: queue=%s, parsed=%s", uuid, cdr.UUID)
	}

	// Enrich CDR with business logic
	if err := p.enricher.Enrich(ctx, cdr); err != nil {
		log.Printf("[CDRProcessor] Enrichment warning for %s: %v", uuid, err)
		// Continue even if enrichment fails
	}

	// Insert into final CDR table
	if err := p.db.InsertCDR(ctx, cdr); err != nil {
		return fmt.Errorf("insert CDR: %w", err)
	}

	log.Printf("[CDRProcessor] Successfully processed CDR %s (caller=%s, dest=%s, duration=%ds)",
		uuid, cdr.CallerIDNumber, cdr.DestinationNumber, cdr.Duration)

	return nil
}

// Stop signals the processor to stop
func (p *CDRProcessor) Stop() {
	<-p.done
}

// CleanupWorker periodically cleans up old processed CDR queue entries
type CleanupWorker struct {
	db              *database.DB
	cleanupInterval time.Duration
	retentionDays   int
	done            chan struct{}
}

// NewCleanupWorker creates a new cleanup worker
func NewCleanupWorker(db *database.DB, cleanupInterval time.Duration, retentionDays int) *CleanupWorker {
	if cleanupInterval == 0 {
		cleanupInterval = 24 * time.Hour // Once per day
	}
	if retentionDays == 0 {
		retentionDays = 7 // Keep for 7 days
	}

	return &CleanupWorker{
		db:              db,
		cleanupInterval: cleanupInterval,
		retentionDays:   retentionDays,
		done:            make(chan struct{}),
	}
}

// Start begins the cleanup worker
func (w *CleanupWorker) Start(ctx context.Context) {
	log.Printf("[CleanupWorker] Starting with interval=%v, retention=%d days",
		w.cleanupInterval, w.retentionDays)

	ticker := time.NewTicker(w.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[CleanupWorker] Shutting down...")
			close(w.done)
			return

		case <-ticker.C:
			if err := w.cleanup(ctx); err != nil {
				log.Printf("[CleanupWorker] Error during cleanup: %v", err)
			}
		}
	}
}

// cleanup removes old processed CDR queue entries
func (w *CleanupWorker) cleanup(ctx context.Context) error {
	deleted, err := w.db.CleanupOldCDRQueue(ctx, w.retentionDays)
	if err != nil {
		return fmt.Errorf("cleanup old cdr queue: %w", err)
	}

	if deleted > 0 {
		log.Printf("[CleanupWorker] Cleaned up %d old CDR queue entries", deleted)
	}

	return nil
}

// Stop signals the cleanup worker to stop
func (w *CleanupWorker) Stop() {
	<-w.done
}
