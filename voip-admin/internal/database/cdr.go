package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/yourusername/high-cc-pbx/voip-admin/internal/models"
)

// InsertCDRQueue inserts a CDR into the processing queue
func (db *DB) InsertCDRQueue(ctx context.Context, uuid, xmlData string) error {
	query := `
		INSERT INTO voip.cdr_queue (uuid, xml_data, received_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (uuid) DO NOTHING
	`

	_, err := db.ExecContext(ctx, query, uuid, xmlData, time.Now())
	if err != nil {
		return fmt.Errorf("insert cdr queue: %w", err)
	}

	return nil
}

// GetPendingCDRs retrieves pending CDR records from the queue for processing
func (db *DB) GetPendingCDRs(ctx context.Context, limit int) ([]*models.CDRQueue, error) {
	query := `
		SELECT id, uuid, xml_data, received_at, processed_at, retry_count, error_message
		FROM voip.cdr_queue
		WHERE processed_at IS NULL
		  AND retry_count < 3
		ORDER BY received_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query pending cdrs: %w", err)
	}
	defer rows.Close()

	var cdrs []*models.CDRQueue
	for rows.Next() {
		var cdr models.CDRQueue
		if err := rows.Scan(
			&cdr.ID, &cdr.UUID, &cdr.XMLData, &cdr.ReceivedAt,
			&cdr.ProcessedAt, &cdr.RetryCount, &cdr.ErrorMessage,
		); err != nil {
			return nil, fmt.Errorf("scan cdr queue: %w", err)
		}
		cdrs = append(cdrs, &cdr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return cdrs, nil
}

// MarkCDRProcessed marks a CDR queue entry as successfully processed
func (db *DB) MarkCDRProcessed(ctx context.Context, id int64) error {
	query := `
		UPDATE voip.cdr_queue
		SET processed_at = $1
		WHERE id = $2
	`

	result, err := db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("mark cdr processed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("cdr queue entry not found: %d", id)
	}

	return nil
}

// MarkCDRFailed marks a CDR queue entry as failed with error message
func (db *DB) MarkCDRFailed(ctx context.Context, id int64, errorMsg string) error {
	query := `
		UPDATE voip.cdr_queue
		SET retry_count = retry_count + 1, error_message = $1
		WHERE id = $2
	`

	result, err := db.ExecContext(ctx, query, errorMsg, id)
	if err != nil {
		return fmt.Errorf("mark cdr failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("cdr queue entry not found: %d", id)
	}

	return nil
}

// InsertCDR inserts a processed CDR into the final table
func (db *DB) InsertCDR(ctx context.Context, cdr *models.CDR) error {
	query := `
		INSERT INTO voip.cdr (
			uuid, caller_id_number, caller_id_name, destination_number,
			context, extension, domain, start_stamp, answer_stamp, end_stamp,
			duration, billsec, holdsec, hangup_cause, hangup_cause_q850,
			sip_hangup_disposition, direction, call_type, queue_id,
			queue_wait_time, agent_extension, record_file, record_duration,
			sip_from_user, sip_to_user, sip_call_id, user_agent,
			read_codec, write_codec, remote_media_ip,
			rtp_audio_in_mos, rtp_audio_in_packet_count, rtp_audio_in_packet_loss,
			rtp_audio_in_jitter_min, rtp_audio_in_jitter_max
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26, $27, $28, $29, $30,
			$31, $32, $33, $34, $35
		)
		ON CONFLICT (uuid) DO NOTHING
		RETURNING id, created_at
	`

	err := db.QueryRowContext(ctx, query,
		cdr.UUID, cdr.CallerIDNumber, cdr.CallerIDName, cdr.DestinationNumber,
		cdr.Context, cdr.Extension, cdr.Domain, cdr.StartStamp, cdr.AnswerStamp, cdr.EndStamp,
		cdr.Duration, cdr.BillSec, cdr.HoldSec, cdr.HangupCause, cdr.HangupCauseQ850,
		cdr.SIPHangupDisp, cdr.Direction, cdr.CallType, cdr.QueueID,
		cdr.QueueWaitTime, cdr.AgentExtension, cdr.RecordFile, cdr.RecordDuration,
		cdr.SIPFromUser, cdr.SIPToUser, cdr.SIPCallID, cdr.UserAgent,
		cdr.ReadCodec, cdr.WriteCodec, cdr.RemoteMediaIP,
		cdr.RTPAudioInMOS, cdr.RTPAudioInPacketCount, cdr.RTPAudioInPacketLoss,
		cdr.RTPAudioInJitterMin, cdr.RTPAudioInJitterMax,
	).Scan(&cdr.ID, &cdr.CreatedAt)

	if err != nil {
		return fmt.Errorf("insert cdr: %w", err)
	}

	return nil
}

// GetCDRByUUID retrieves a CDR by UUID
func (db *DB) GetCDRByUUID(ctx context.Context, uuid string) (*models.CDR, error) {
	query := `
		SELECT
			id, uuid, caller_id_number, caller_id_name, destination_number,
			context, extension, domain, start_stamp, answer_stamp, end_stamp,
			duration, billsec, holdsec, hangup_cause, hangup_cause_q850,
			sip_hangup_disposition, direction, call_type, queue_id,
			queue_wait_time, agent_extension, record_file, record_duration,
			sip_from_user, sip_to_user, sip_call_id, user_agent,
			read_codec, write_codec, remote_media_ip,
			rtp_audio_in_mos, rtp_audio_in_packet_count, rtp_audio_in_packet_loss,
			rtp_audio_in_jitter_min, rtp_audio_in_jitter_max, created_at
		FROM voip.cdr
		WHERE uuid = $1
	`

	var cdr models.CDR
	err := db.QueryRowContext(ctx, query, uuid).Scan(
		&cdr.ID, &cdr.UUID, &cdr.CallerIDNumber, &cdr.CallerIDName, &cdr.DestinationNumber,
		&cdr.Context, &cdr.Extension, &cdr.Domain, &cdr.StartStamp, &cdr.AnswerStamp, &cdr.EndStamp,
		&cdr.Duration, &cdr.BillSec, &cdr.HoldSec, &cdr.HangupCause, &cdr.HangupCauseQ850,
		&cdr.SIPHangupDisp, &cdr.Direction, &cdr.CallType, &cdr.QueueID,
		&cdr.QueueWaitTime, &cdr.AgentExtension, &cdr.RecordFile, &cdr.RecordDuration,
		&cdr.SIPFromUser, &cdr.SIPToUser, &cdr.SIPCallID, &cdr.UserAgent,
		&cdr.ReadCodec, &cdr.WriteCodec, &cdr.RemoteMediaIP,
		&cdr.RTPAudioInMOS, &cdr.RTPAudioInPacketCount, &cdr.RTPAudioInPacketLoss,
		&cdr.RTPAudioInJitterMin, &cdr.RTPAudioInJitterMax, &cdr.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("cdr not found: %s", uuid)
	}
	if err != nil {
		return nil, fmt.Errorf("query cdr: %w", err)
	}

	return &cdr, nil
}

// ListCDRs retrieves CDRs with pagination and filtering
func (db *DB) ListCDRs(ctx context.Context, req *models.CDRListRequest) (*models.CDRListResponse, error) {
	// Build WHERE clause
	var conditions []string
	var args []interface{}
	argPos := 1

	if req.StartDate != nil {
		conditions = append(conditions, fmt.Sprintf("start_stamp >= $%d", argPos))
		args = append(args, *req.StartDate)
		argPos++
	}

	if req.EndDate != nil {
		conditions = append(conditions, fmt.Sprintf("start_stamp <= $%d", argPos))
		args = append(args, *req.EndDate)
		argPos++
	}

	if req.CallerIDNumber != nil {
		conditions = append(conditions, fmt.Sprintf("caller_id_number LIKE $%d", argPos))
		args = append(args, "%"+*req.CallerIDNumber+"%")
		argPos++
	}

	if req.DestNumber != nil {
		conditions = append(conditions, fmt.Sprintf("destination_number LIKE $%d", argPos))
		args = append(args, "%"+*req.DestNumber+"%")
		argPos++
	}

	if req.Direction != nil {
		conditions = append(conditions, fmt.Sprintf("direction = $%d", argPos))
		args = append(args, *req.Direction)
		argPos++
	}

	if req.HangupCause != nil {
		conditions = append(conditions, fmt.Sprintf("hangup_cause = $%d", argPos))
		args = append(args, *req.HangupCause)
		argPos++
	}

	if req.QueueID != nil {
		conditions = append(conditions, fmt.Sprintf("queue_id = $%d", argPos))
		args = append(args, *req.QueueID)
		argPos++
	}

	if req.MinDuration != nil {
		conditions = append(conditions, fmt.Sprintf("duration >= $%d", argPos))
		args = append(args, *req.MinDuration)
		argPos++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM voip.cdr
		%s
	`, whereClause)

	var total int64
	if err := db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count cdrs: %w", err)
	}

	// Fetch CDRs
	offset := (req.Page - 1) * req.PerPage
	args = append(args, req.PerPage, offset)

	query := fmt.Sprintf(`
		SELECT
			id, uuid, caller_id_number, caller_id_name, destination_number,
			context, extension, domain, start_stamp, answer_stamp, end_stamp,
			duration, billsec, holdsec, hangup_cause, hangup_cause_q850,
			sip_hangup_disposition, direction, call_type, queue_id,
			queue_wait_time, agent_extension, record_file, record_duration,
			sip_from_user, sip_to_user, sip_call_id, user_agent,
			read_codec, write_codec, remote_media_ip,
			rtp_audio_in_mos, rtp_audio_in_packet_count, rtp_audio_in_packet_loss,
			rtp_audio_in_jitter_min, rtp_audio_in_jitter_max, created_at
		FROM voip.cdr
		%s
		ORDER BY start_stamp DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argPos, argPos+1)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query cdrs: %w", err)
	}
	defer rows.Close()

	var cdrs []*models.CDR
	for rows.Next() {
		var cdr models.CDR
		if err := rows.Scan(
			&cdr.ID, &cdr.UUID, &cdr.CallerIDNumber, &cdr.CallerIDName, &cdr.DestinationNumber,
			&cdr.Context, &cdr.Extension, &cdr.Domain, &cdr.StartStamp, &cdr.AnswerStamp, &cdr.EndStamp,
			&cdr.Duration, &cdr.BillSec, &cdr.HoldSec, &cdr.HangupCause, &cdr.HangupCauseQ850,
			&cdr.SIPHangupDisp, &cdr.Direction, &cdr.CallType, &cdr.QueueID,
			&cdr.QueueWaitTime, &cdr.AgentExtension, &cdr.RecordFile, &cdr.RecordDuration,
			&cdr.SIPFromUser, &cdr.SIPToUser, &cdr.SIPCallID, &cdr.UserAgent,
			&cdr.ReadCodec, &cdr.WriteCodec, &cdr.RemoteMediaIP,
			&cdr.RTPAudioInMOS, &cdr.RTPAudioInPacketCount, &cdr.RTPAudioInPacketLoss,
			&cdr.RTPAudioInJitterMin, &cdr.RTPAudioInJitterMax, &cdr.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan cdr: %w", err)
		}
		cdrs = append(cdrs, &cdr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return &models.CDRListResponse{
		CDRs:    cdrs,
		Total:   total,
		Page:    req.Page,
		PerPage: req.PerPage,
	}, nil
}

// GetCDRStats retrieves CDR statistics for a given time period
func (db *DB) GetCDRStats(ctx context.Context, startDate, endDate time.Time) (*models.CDRStats, error) {
	query := `
		SELECT
			COUNT(*) as total_calls,
			COUNT(CASE WHEN answer_stamp IS NOT NULL THEN 1 END) as answered_calls,
			COUNT(CASE WHEN answer_stamp IS NULL THEN 1 END) as missed_calls,
			COALESCE(AVG(duration), 0) as avg_duration,
			COALESCE(AVG(billsec), 0) as avg_billsec,
			COALESCE(SUM(duration), 0) as total_duration,
			COALESCE(SUM(billsec), 0) as total_billsec
		FROM voip.cdr
		WHERE start_stamp >= $1 AND start_stamp <= $2
	`

	var stats models.CDRStats
	err := db.QueryRowContext(ctx, query, startDate, endDate).Scan(
		&stats.TotalCalls,
		&stats.AnsweredCalls,
		&stats.MissedCalls,
		&stats.AverageDuration,
		&stats.AverageBillSec,
		&stats.TotalDuration,
		&stats.TotalBillSec,
	)

	if err != nil {
		return nil, fmt.Errorf("query cdr stats: %w", err)
	}

	return &stats, nil
}

// CleanupOldCDRQueue removes processed CDR queue entries older than specified days
func (db *DB) CleanupOldCDRQueue(ctx context.Context, daysOld int) (int64, error) {
	query := `
		DELETE FROM voip.cdr_queue
		WHERE processed_at IS NOT NULL
		  AND processed_at < NOW() - INTERVAL '1 day' * $1
	`

	result, err := db.ExecContext(ctx, query, daysOld)
	if err != nil {
		return 0, fmt.Errorf("cleanup old cdr queue: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get rows affected: %w", err)
	}

	return rowsAffected, nil
}
