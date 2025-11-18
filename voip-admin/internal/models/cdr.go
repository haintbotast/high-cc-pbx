package models

import "time"

// CDRQueue represents a CDR record in the processing queue
type CDRQueue struct {
	ID           int64     `json:"id" db:"id"`
	UUID         string    `json:"uuid" db:"uuid"`
	XMLData      string    `json:"xml_data" db:"xml_data"`
	ReceivedAt   time.Time `json:"received_at" db:"received_at"`
	ProcessedAt  *time.Time `json:"processed_at,omitempty" db:"processed_at"`
	RetryCount   int       `json:"retry_count" db:"retry_count"`
	ErrorMessage *string   `json:"error_message,omitempty" db:"error_message"`
}

// CDR represents a processed call detail record
type CDR struct {
	ID                 int64      `json:"id" db:"id"`
	UUID               string     `json:"uuid" db:"uuid"`

	// Call Participants
	CallerIDNumber     string     `json:"caller_id_number" db:"caller_id_number"`
	CallerIDName       string     `json:"caller_id_name" db:"caller_id_name"`
	DestinationNumber  string     `json:"destination_number" db:"destination_number"`

	// Context Information
	Context            string     `json:"context" db:"context"`
	Extension          string     `json:"extension" db:"extension"`
	Domain             string     `json:"domain" db:"domain"`

	// Timing Information
	StartStamp         time.Time  `json:"start_stamp" db:"start_stamp"`
	AnswerStamp        *time.Time `json:"answer_stamp,omitempty" db:"answer_stamp"`
	EndStamp           time.Time  `json:"end_stamp" db:"end_stamp"`
	Duration           int        `json:"duration" db:"duration"`           // Total duration in seconds
	BillSec            int        `json:"billsec" db:"billsec"`             // Billable seconds (after answer)
	HoldSec            int        `json:"holdsec" db:"holdsec"`             // Time on hold

	// Call Result
	HangupCause        string     `json:"hangup_cause" db:"hangup_cause"`
	HangupCauseQ850    *int       `json:"hangup_cause_q850,omitempty" db:"hangup_cause_q850"`
	SIPHangupDisp      *string    `json:"sip_hangup_disposition,omitempty" db:"sip_hangup_disposition"`

	// Call Direction & Type
	Direction          string     `json:"direction" db:"direction"`         // inbound, outbound, internal
	CallType           string     `json:"call_type" db:"call_type"`         // queue, direct, ivr, conference

	// Queue Specific (if applicable)
	QueueID            *int64     `json:"queue_id,omitempty" db:"queue_id"`
	QueueWaitTime      *int       `json:"queue_wait_time,omitempty" db:"queue_wait_time"`
	AgentExtension     *string    `json:"agent_extension,omitempty" db:"agent_extension"`

	// Recording
	RecordFile         *string    `json:"record_file,omitempty" db:"record_file"`
	RecordDuration     *int       `json:"record_duration,omitempty" db:"record_duration"`

	// SIP Information
	SIPFromUser        *string    `json:"sip_from_user,omitempty" db:"sip_from_user"`
	SIPToUser          *string    `json:"sip_to_user,omitempty" db:"sip_to_user"`
	SIPCallID          *string    `json:"sip_call_id,omitempty" db:"sip_call_id"`
	UserAgent          *string    `json:"user_agent,omitempty" db:"user_agent"`

	// Media Information
	ReadCodec          *string    `json:"read_codec,omitempty" db:"read_codec"`
	WriteCodec         *string    `json:"write_codec,omitempty" db:"write_codec"`
	RemoteMediaIP      *string    `json:"remote_media_ip,omitempty" db:"remote_media_ip"`

	// Network Quality
	RTPAudioInMOS      *float64   `json:"rtp_audio_in_mos,omitempty" db:"rtp_audio_in_mos"`
	RTPAudioInPacketCount *int    `json:"rtp_audio_in_packet_count,omitempty" db:"rtp_audio_in_packet_count"`
	RTPAudioInPacketLoss  *int    `json:"rtp_audio_in_packet_loss,omitempty" db:"rtp_audio_in_packet_loss"`
	RTPAudioInJitterMin   *int    `json:"rtp_audio_in_jitter_min,omitempty" db:"rtp_audio_in_jitter_min"`
	RTPAudioInJitterMax   *int    `json:"rtp_audio_in_jitter_max,omitempty" db:"rtp_audio_in_jitter_max"`

	// Timestamps
	CreatedAt          time.Time  `json:"created_at" db:"created_at"`
}

// CDRIngest represents the initial CDR data received from FreeSWITCH
type CDRIngest struct {
	UUID    string `json:"uuid" validate:"required"`
	XMLData string `json:"xml_data" validate:"required"`
}

// CDRListRequest represents parameters for listing CDRs
type CDRListRequest struct {
	StartDate       *time.Time `json:"start_date,omitempty"`
	EndDate         *time.Time `json:"end_date,omitempty"`
	CallerIDNumber  *string    `json:"caller_id_number,omitempty"`
	DestNumber      *string    `json:"destination_number,omitempty"`
	Direction       *string    `json:"direction,omitempty" validate:"omitempty,oneof=inbound outbound internal"`
	HangupCause     *string    `json:"hangup_cause,omitempty"`
	QueueID         *int64     `json:"queue_id,omitempty"`
	MinDuration     *int       `json:"min_duration,omitempty"`
	Page            int        `json:"page" validate:"min=1"`
	PerPage         int        `json:"per_page" validate:"min=1,max=1000"`
}

// CDRListResponse represents paginated CDR list
type CDRListResponse struct {
	CDRs    []*CDR `json:"cdrs"`
	Total   int64  `json:"total"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
}

// CDRStats represents CDR statistics
type CDRStats struct {
	TotalCalls       int64   `json:"total_calls"`
	AnsweredCalls    int64   `json:"answered_calls"`
	MissedCalls      int64   `json:"missed_calls"`
	AverageDuration  float64 `json:"average_duration"`
	AverageBillSec   float64 `json:"average_billsec"`
	TotalDuration    int64   `json:"total_duration"`
	TotalBillSec     int64   `json:"total_billsec"`
}
