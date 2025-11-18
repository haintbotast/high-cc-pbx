package models

import "time"

// Queue represents a call queue
type Queue struct {
	ID                 int64     `json:"id" db:"id"`
	Name               string    `json:"name" db:"name"`
	Extension          string    `json:"extension" db:"extension"`
	DomainID           int64     `json:"domain_id" db:"domain_id"`
	Strategy           string    `json:"strategy" db:"strategy"` // ring-all, longest-idle-agent, round-robin, etc.
	Moh                string    `json:"moh" db:"moh"`           // Music on hold
	RecordTemplate     string    `json:"record_template,omitempty" db:"record_template"`
	TimeBaseScore      string    `json:"time_base_score" db:"time_base_score"` // queue, system
	MaxWaitTime        int       `json:"max_wait_time" db:"max_wait_time"`     // seconds
	MaxWaitTimeNoAgent int       `json:"max_wait_time_no_agent" db:"max_wait_time_no_agent"`
	TierRulesApply     bool      `json:"tier_rules_apply" db:"tier_rules_apply"`
	TierRuleWaitSecond int       `json:"tier_rule_wait_second" db:"tier_rule_wait_second"`
	DiscardAbandonedAfter int    `json:"discard_abandoned_after" db:"discard_abandoned_after"`
	AbandonedResumeAllowed bool  `json:"abandoned_resume_allowed" db:"abandoned_resume_allowed"`
	Active             bool      `json:"active" db:"active"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" db:"updated_at"`

	// Joined fields
	Domain             string    `json:"domain,omitempty" db:"domain"`
}

// QueueAgent represents an agent assigned to a queue
type QueueAgent struct {
	ID           int64     `json:"id" db:"id"`
	QueueID      int64     `json:"queue_id" db:"queue_id"`
	ExtensionID  int64     `json:"extension_id" db:"extension_id"`
	State        string    `json:"state" db:"state"`         // Available, On Break, Logged Out
	Status       string    `json:"status" db:"status"`       // Waiting, Receiving, In a queue call
	Tier         int       `json:"tier" db:"tier"`           // Priority tier (1-10)
	Position     int       `json:"position" db:"position"`   // Position within tier
	Active       bool      `json:"active" db:"active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`

	// Joined fields
	Extension    string    `json:"extension,omitempty" db:"extension"`
	DisplayName  string    `json:"display_name,omitempty" db:"display_name"`
}

// QueueCreateRequest represents a request to create a queue
type QueueCreateRequest struct {
	Name                   string `json:"name" validate:"required,min=1,max=255"`
	Extension              string `json:"extension" validate:"required,min=3,max=20"`
	DomainID               int64  `json:"domain_id" validate:"required,gt=0"`
	Strategy               string `json:"strategy" validate:"required,oneof=ring-all longest-idle-agent round-robin top-down agent-with-least-talk-time agent-with-fewest-calls sequentially-by-agent-order random"`
	Moh                    string `json:"moh" validate:"required"`
	RecordTemplate         string `json:"record_template,omitempty"`
	TimeBaseScore          string `json:"time_base_score" validate:"required,oneof=queue system"`
	MaxWaitTime            int    `json:"max_wait_time" validate:"min=10,max=3600"`
	MaxWaitTimeNoAgent     int    `json:"max_wait_time_no_agent" validate:"min=5,max=300"`
	TierRulesApply         bool   `json:"tier_rules_apply"`
	TierRuleWaitSecond     int    `json:"tier_rule_wait_second" validate:"min=0,max=600"`
	DiscardAbandonedAfter  int    `json:"discard_abandoned_after" validate:"min=10,max=300"`
	AbandonedResumeAllowed bool   `json:"abandoned_resume_allowed"`
	Active                 bool   `json:"active"`
}

// QueueUpdateRequest represents a request to update a queue
type QueueUpdateRequest struct {
	Name                   *string `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Strategy               *string `json:"strategy,omitempty" validate:"omitempty,oneof=ring-all longest-idle-agent round-robin top-down agent-with-least-talk-time agent-with-fewest-calls sequentially-by-agent-order random"`
	Moh                    *string `json:"moh,omitempty"`
	RecordTemplate         *string `json:"record_template,omitempty"`
	TimeBaseScore          *string `json:"time_base_score,omitempty" validate:"omitempty,oneof=queue system"`
	MaxWaitTime            *int    `json:"max_wait_time,omitempty" validate:"omitempty,min=10,max=3600"`
	MaxWaitTimeNoAgent     *int    `json:"max_wait_time_no_agent,omitempty" validate:"omitempty,min=5,max=300"`
	TierRulesApply         *bool   `json:"tier_rules_apply,omitempty"`
	TierRuleWaitSecond     *int    `json:"tier_rule_wait_second,omitempty" validate:"omitempty,min=0,max=600"`
	DiscardAbandonedAfter  *int    `json:"discard_abandoned_after,omitempty" validate:"omitempty,min=10,max=300"`
	AbandonedResumeAllowed *bool   `json:"abandoned_resume_allowed,omitempty"`
	Active                 *bool   `json:"active,omitempty"`
}

// QueueAgentCreateRequest represents a request to add an agent to a queue
type QueueAgentCreateRequest struct {
	QueueID     int64  `json:"queue_id" validate:"required,gt=0"`
	ExtensionID int64  `json:"extension_id" validate:"required,gt=0"`
	Tier        int    `json:"tier" validate:"min=1,max=10"`
	Position    int    `json:"position" validate:"min=1,max=100"`
	State       string `json:"state" validate:"required,oneof=Available 'On Break' 'Logged Out'"`
	Active      bool   `json:"active"`
}

// QueueAgentUpdateRequest represents a request to update a queue agent
type QueueAgentUpdateRequest struct {
	Tier     *int    `json:"tier,omitempty" validate:"omitempty,min=1,max=10"`
	Position *int    `json:"position,omitempty" validate:"omitempty,min=1,max=100"`
	State    *string `json:"state,omitempty" validate:"omitempty,oneof=Available 'On Break' 'Logged Out'"`
	Status   *string `json:"status,omitempty" validate:"omitempty,oneof=Waiting Receiving 'In a queue call'"`
	Active   *bool   `json:"active,omitempty"`
}
