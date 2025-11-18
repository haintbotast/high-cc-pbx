package models

import "time"

// Extension represents a VoIP extension/user
type Extension struct {
	ID               int64     `json:"id" db:"id"`
	DomainID         int64     `json:"domain_id" db:"domain_id"`
	Extension        string    `json:"extension" db:"extension"`
	Type             string    `json:"type" db:"type"` // user, queue, ivr, conference
	DisplayName      string    `json:"display_name" db:"display_name"`
	Email            string    `json:"email,omitempty" db:"email"`
	SIPPassword      string    `json:"-" db:"sip_password"`       // Never expose in JSON
	SIPHA1           string    `json:"-" db:"sip_ha1"`            // MD5 hash for auth
	SIPHA1B          string    `json:"-" db:"sip_ha1b"`           // MD5 hash variant
	VMPassword       string    `json:"vm_password,omitempty" db:"vm_password"`
	VMEmail          string    `json:"vm_email,omitempty" db:"vm_email"`
	Active           bool      `json:"active" db:"active"`
	MaxConcurrent    int       `json:"max_concurrent" db:"max_concurrent"`
	CallTimeout      int       `json:"call_timeout" db:"call_timeout"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`

	// Joined fields from domains table
	Domain           string    `json:"domain,omitempty" db:"domain"`
}

// ExtensionCreateRequest represents a request to create a new extension
type ExtensionCreateRequest struct {
	DomainID      int64  `json:"domain_id" validate:"required,gt=0"`
	Extension     string `json:"extension" validate:"required,min=3,max=20"`
	Type          string `json:"type" validate:"required,oneof=user queue ivr conference"`
	DisplayName   string `json:"display_name" validate:"required,min=1,max=255"`
	Email         string `json:"email,omitempty" validate:"omitempty,email"`
	SIPPassword   string `json:"sip_password" validate:"required,min=8,max=128"`
	VMPassword    string `json:"vm_password,omitempty" validate:"omitempty,len=4"`
	VMEmail       string `json:"vm_email,omitempty" validate:"omitempty,email"`
	Active        bool   `json:"active"`
	MaxConcurrent int    `json:"max_concurrent" validate:"min=1,max=100"`
	CallTimeout   int    `json:"call_timeout" validate:"min=10,max=300"`
}

// ExtensionUpdateRequest represents a request to update an extension
type ExtensionUpdateRequest struct {
	DisplayName   *string `json:"display_name,omitempty" validate:"omitempty,min=1,max=255"`
	Email         *string `json:"email,omitempty" validate:"omitempty,email"`
	VMPassword    *string `json:"vm_password,omitempty" validate:"omitempty,len=4"`
	VMEmail       *string `json:"vm_email,omitempty" validate:"omitempty,email"`
	Active        *bool   `json:"active,omitempty"`
	MaxConcurrent *int    `json:"max_concurrent,omitempty" validate:"omitempty,min=1,max=100"`
	CallTimeout   *int    `json:"call_timeout,omitempty" validate:"omitempty,min=10,max=300"`
}

// ExtensionPasswordUpdate represents a request to change extension password
type ExtensionPasswordUpdate struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=128"`
}

// Domain represents a SIP domain
type Domain struct {
	ID        int64     `json:"id" db:"id"`
	Domain    string    `json:"domain" db:"domain"`
	Active    bool      `json:"active" db:"active"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ExtensionListResponse represents paginated extension list
type ExtensionListResponse struct {
	Extensions []*Extension `json:"extensions"`
	Total      int64        `json:"total"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
}
