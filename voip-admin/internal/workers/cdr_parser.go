package workers

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/yourusername/high-cc-pbx/voip-admin/internal/models"
)

// FreeSwitchCDR represents the raw CDR XML structure from FreeSWITCH
type FreeSwitchCDR struct {
	XMLName xml.Name `xml:"cdr"`
	Core    struct {
		UUID         string `xml:"uuid"`
		CoreUUID     string `xml:"core-uuid"`
		SwitchName   string `xml:"switchname"`
	} `xml:"core-uuid"`
	Variables Variables `xml:"variables"`
	App       struct {
		Log []AppLog `xml:"log"`
	} `xml:"app_log"`
	Callflow Callflow `xml:"callflow"`
}

// Variables represents FreeSWITCH CDR variables
type Variables struct {
	// Call identification
	UUID                  string `xml:"uuid"`
	Direction             string `xml:"direction"` // inbound, outbound
	CallUUID              string `xml:"call_uuid"`

	// Caller information
	CallerIDNumber        string `xml:"caller_id_number"`
	CallerIDName          string `xml:"caller_id_name"`
	CallerOrigIDNumber    string `xml:"origination_caller_id_number"`
	CallerOrigIDName      string `xml:"origination_caller_id_name"`

	// Destination information
	DestinationNumber     string `xml:"destination_number"`
	DialedUser            string `xml:"dialed_user"`
	DialedDomain          string `xml:"dialed_domain"`

	// Context
	Context               string `xml:"context"`
	DialplanExtension     string `xml:"dialplan_extension"`

	// Domain
	DomainName            string `xml:"domain_name"`
	SIPFromHost           string `xml:"sip_from_host"`
	SIPToHost             string `xml:"sip_to_host"`

	// Timing (epoch seconds)
	StartEpoch            string `xml:"start_epoch"`
	AnswerEpoch           string `xml:"answer_epoch"`
	BridgeEpoch           string `xml:"bridge_epoch"`
	EndEpoch              string `xml:"end_epoch"`
	Duration              string `xml:"duration"`
	BillSec               string `xml:"billsec"`
	ProgressSec           string `xml:"progresssec"`
	WaitSec               string `xml:"waitsec"`
	HoldSec               string `xml:"holdsec"`

	// Hangup cause
	HangupCause           string `xml:"hangup_cause"`
	HangupCauseQ850       string `xml:"hangup_cause_q850"`
	SIPHangupDisposition  string `xml:"sip_hangup_disposition"`

	// SIP information
	SIPFromUser           string `xml:"sip_from_user"`
	SIPToUser             string `xml:"sip_to_user"`
	SIPCallID             string `xml:"sip_call_id"`
	SIPUserAgent          string `xml:"sip_user_agent"`

	// Media codec
	ReadCodec             string `xml:"read_codec"`
	ReadRate              string `xml:"read_rate"`
	WriteCodec            string `xml:"write_codec"`
	WriteRate             string `xml:"write_rate"`

	// Network
	RemoteMediaIP         string `xml:"remote_media_ip"`
	LocalMediaIP          string `xml:"local_media_ip"`
	NetworkAddr           string `xml:"network_addr"`

	// RTP statistics (audio quality)
	RTPAudioInMOS         string `xml:"rtp_audio_in_mos"`
	RTPAudioInPacketCount string `xml:"rtp_audio_in_packet_count"`
	RTPAudioInMediaBytes  string `xml:"rtp_audio_in_media_bytes"`
	RTPAudioInSkipPacketCount string `xml:"rtp_audio_in_skip_packet_count"`
	RTPAudioInJitterMinVariance string `xml:"rtp_audio_in_jitter_min_variance"`
	RTPAudioInJitterMaxVariance string `xml:"rtp_audio_in_jitter_max_variance"`

	// Recording
	RecordingFile         string `xml:"recording_file"`
	RecordSeconds         string `xml:"record_seconds"`

	// Queue specific (if present)
	CCQueue               string `xml:"cc_queue"`
	CCQueueJoinedEpoch    string `xml:"cc_queue_joined_epoch"`
	CCQueueAnsweredEpoch  string `xml:"cc_queue_answered_epoch"`
	CCAgent               string `xml:"cc_agent"`
	CCAgentEpoch          string `xml:"cc_agent_epoch"`
}

// AppLog represents application log entry
type AppLog struct {
	App  string `xml:"app,attr"`
	Data string `xml:"data,attr"`
}

// Callflow represents the call flow information
type Callflow struct {
	DialplanExtension string `xml:"extension>name"`
	CallerProfile     struct {
		Username           string `xml:"username"`
		DialPlan           string `xml:"dialplan"`
		CallerIDName       string `xml:"caller_id_name"`
		CallerIDNumber     string `xml:"caller_id_number"`
		NetworkAddr        string `xml:"network_addr"`
		ANI                string `xml:"ani"`
		ANIII              string `xml:"aniii"`
		RDNIS              string `xml:"rdnis"`
		DestinationNumber  string `xml:"destination_number"`
		Context            string `xml:"context"`
		ChanName           string `xml:"chan_name"`
	} `xml:"caller_profile"`
}

// ParseCDRXML parses FreeSWITCH CDR XML into a CDR model
func ParseCDRXML(xmlData string) (*models.CDR, error) {
	var fsCDR FreeSwitchCDR
	if err := xml.Unmarshal([]byte(xmlData), &fsCDR); err != nil {
		return nil, fmt.Errorf("unmarshal XML: %w", err)
	}

	cdr := &models.CDR{}

	// UUID
	cdr.UUID = fsCDR.Variables.UUID
	if cdr.UUID == "" {
		return nil, fmt.Errorf("missing UUID in CDR")
	}

	// Call participants
	cdr.CallerIDNumber = fsCDR.Variables.CallerIDNumber
	cdr.CallerIDName = fsCDR.Variables.CallerIDName
	cdr.DestinationNumber = fsCDR.Variables.DestinationNumber

	// Context
	cdr.Context = fsCDR.Variables.Context
	cdr.Extension = fsCDR.Variables.DialedUser
	if cdr.Extension == "" {
		cdr.Extension = fsCDR.Variables.DestinationNumber
	}
	cdr.Domain = fsCDR.Variables.DomainName
	if cdr.Domain == "" {
		cdr.Domain = fsCDR.Variables.DialedDomain
	}

	// Timing
	if err := parseTiming(cdr, &fsCDR.Variables); err != nil {
		return nil, fmt.Errorf("parse timing: %w", err)
	}

	// Hangup cause
	cdr.HangupCause = fsCDR.Variables.HangupCause
	if q850 := fsCDR.Variables.HangupCauseQ850; q850 != "" {
		if val, err := strconv.Atoi(q850); err == nil {
			cdr.HangupCauseQ850 = &val
		}
	}
	if sip := fsCDR.Variables.SIPHangupDisposition; sip != "" {
		cdr.SIPHangupDisp = &sip
	}

	// Direction
	cdr.Direction = determineDirection(fsCDR.Variables.Direction, fsCDR.Variables.DestinationNumber)

	// SIP information
	if sipFromUser := fsCDR.Variables.SIPFromUser; sipFromUser != "" {
		cdr.SIPFromUser = &sipFromUser
	}
	if sipToUser := fsCDR.Variables.SIPToUser; sipToUser != "" {
		cdr.SIPToUser = &sipToUser
	}
	if sipCallID := fsCDR.Variables.SIPCallID; sipCallID != "" {
		cdr.SIPCallID = &sipCallID
	}
	if userAgent := fsCDR.Variables.SIPUserAgent; userAgent != "" {
		cdr.UserAgent = &userAgent
	}

	// Media codec
	if readCodec := fsCDR.Variables.ReadCodec; readCodec != "" {
		cdr.ReadCodec = &readCodec
	}
	if writeCodec := fsCDR.Variables.WriteCodec; writeCodec != "" {
		cdr.WriteCodec = &writeCodec
	}
	if remoteIP := fsCDR.Variables.RemoteMediaIP; remoteIP != "" {
		cdr.RemoteMediaIP = &remoteIP
	}

	// RTP statistics
	parseRTPStats(cdr, &fsCDR.Variables)

	// Recording
	if recFile := fsCDR.Variables.RecordingFile; recFile != "" {
		cdr.RecordFile = &recFile
		if recSec := fsCDR.Variables.RecordSeconds; recSec != "" {
			if val, err := strconv.Atoi(recSec); err == nil {
				cdr.RecordDuration = &val
			}
		}
	}

	// Queue information
	parseQueueInfo(cdr, &fsCDR.Variables)

	return cdr, nil
}

// parseTiming parses timing information from CDR variables
func parseTiming(cdr *models.CDR, vars *Variables) error {
	// Start time (required)
	if vars.StartEpoch == "" {
		return fmt.Errorf("missing start_epoch")
	}
	startSec, err := strconv.ParseInt(vars.StartEpoch, 10, 64)
	if err != nil {
		return fmt.Errorf("parse start_epoch: %w", err)
	}
	cdr.StartStamp = time.Unix(startSec, 0)

	// Answer time (optional)
	if vars.AnswerEpoch != "" && vars.AnswerEpoch != "0" {
		answerSec, err := strconv.ParseInt(vars.AnswerEpoch, 10, 64)
		if err == nil {
			answerTime := time.Unix(answerSec, 0)
			cdr.AnswerStamp = &answerTime
		}
	}

	// End time (required)
	if vars.EndEpoch == "" {
		return fmt.Errorf("missing end_epoch")
	}
	endSec, err := strconv.ParseInt(vars.EndEpoch, 10, 64)
	if err != nil {
		return fmt.Errorf("parse end_epoch: %w", err)
	}
	cdr.EndStamp = time.Unix(endSec, 0)

	// Duration (total)
	if vars.Duration != "" {
		if val, err := strconv.Atoi(vars.Duration); err == nil {
			cdr.Duration = val
		}
	} else {
		cdr.Duration = int(cdr.EndStamp.Sub(cdr.StartStamp).Seconds())
	}

	// Billable seconds (after answer)
	if vars.BillSec != "" {
		if val, err := strconv.Atoi(vars.BillSec); err == nil {
			cdr.BillSec = val
		}
	}

	// Hold seconds
	if vars.HoldSec != "" {
		if val, err := strconv.Atoi(vars.HoldSec); err == nil {
			cdr.HoldSec = val
		}
	}

	return nil
}

// parseRTPStats parses RTP quality statistics
func parseRTPStats(cdr *models.CDR, vars *Variables) {
	if mos := vars.RTPAudioInMOS; mos != "" {
		if val, err := strconv.ParseFloat(mos, 64); err == nil {
			cdr.RTPAudioInMOS = &val
		}
	}

	if count := vars.RTPAudioInPacketCount; count != "" {
		if val, err := strconv.Atoi(count); err == nil {
			cdr.RTPAudioInPacketCount = &val
		}
	}

	if skip := vars.RTPAudioInSkipPacketCount; skip != "" {
		if val, err := strconv.Atoi(skip); err == nil {
			cdr.RTPAudioInPacketLoss = &val
		}
	}

	if jitterMin := vars.RTPAudioInJitterMinVariance; jitterMin != "" {
		if val, err := strconv.Atoi(jitterMin); err == nil {
			cdr.RTPAudioInJitterMin = &val
		}
	}

	if jitterMax := vars.RTPAudioInJitterMaxVariance; jitterMax != "" {
		if val, err := strconv.Atoi(jitterMax); err == nil {
			cdr.RTPAudioInJitterMax = &val
		}
	}
}

// parseQueueInfo parses queue-specific information
func parseQueueInfo(cdr *models.CDR, vars *Variables) {
	if queue := vars.CCQueue; queue != "" {
		cdr.CallType = "queue"

		// Calculate queue wait time
		if joinedEpoch := vars.CCQueueJoinedEpoch; joinedEpoch != "" {
			if answeredEpoch := vars.CCQueueAnsweredEpoch; answeredEpoch != "" {
				joined, err1 := strconv.ParseInt(joinedEpoch, 10, 64)
				answered, err2 := strconv.ParseInt(answeredEpoch, 10, 64)
				if err1 == nil && err2 == nil {
					waitTime := int(answered - joined)
					cdr.QueueWaitTime = &waitTime
				}
			}
		}

		// Agent extension
		if agent := vars.CCAgent; agent != "" {
			cdr.AgentExtension = &agent
		}
	} else {
		cdr.CallType = "direct"
	}
}

// determineDirection determines call direction
func determineDirection(fsDirection, destNumber string) string {
	// Check FreeSWITCH direction variable first
	if fsDirection == "inbound" || fsDirection == "outbound" {
		return fsDirection
	}

	// Heuristic: if destination is 4-digit, it's internal
	// if destination is 10+ digits, it's outbound
	if len(destNumber) == 4 {
		return "internal"
	} else if len(destNumber) >= 10 {
		return "outbound"
	}

	// Check if starts with specific prefixes
	if strings.HasPrefix(destNumber, "+") || strings.HasPrefix(destNumber, "00") {
		return "outbound"
	}

	// Default to internal
	return "internal"
}
