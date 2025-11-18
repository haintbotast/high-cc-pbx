package xmlcurl

import (
	"fmt"
	"net/http"
	"strings"
)

// ParseDirectoryRequest parses FreeSWITCH directory POST parameters
func ParseDirectoryRequest(r *http.Request) (*DirectoryRequest, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("parse form: %w", err)
	}

	req := &DirectoryRequest{
		Section:  r.FormValue("section"),
		TagName:  r.FormValue("tag_name"),
		KeyName:  r.FormValue("key_name"),
		KeyValue: r.FormValue("key_value"),
		User:     r.FormValue("user"),
		Domain:   r.FormValue("domain"),
		Action:   r.FormValue("action"),
		SIPAuth:  r.FormValue("sip_auth"),
		Purpose:  r.FormValue("purpose"),
	}

	// Normalize domain (remove port if present)
	if idx := strings.Index(req.Domain, ":"); idx != -1 {
		req.Domain = req.Domain[:idx]
	}

	return req, nil
}

// ParseDialplanRequest parses FreeSWITCH dialplan POST parameters
func ParseDialplanRequest(r *http.Request) (*DialplanRequest, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("parse form: %w", err)
	}

	req := &DialplanRequest{
		Section:           r.FormValue("section"),
		Context:           r.FormValue("Caller-Context"),
		CallerIDNumber:    r.FormValue("Caller-Caller-ID-Number"),
		CallerIDName:      r.FormValue("Caller-Caller-ID-Name"),
		DestinationNumber: r.FormValue("Caller-Destination-Number"),
		Domain:            r.FormValue("variable_domain_name"),
		NetworkAddr:       r.FormValue("Caller-Network-Addr"),
		ChannelName:       r.FormValue("Caller-Channel-Name"),
		UUID:              r.FormValue("Caller-Unique-ID"),
	}

	// Fallback for domain if not in variable_domain_name
	if req.Domain == "" {
		req.Domain = r.FormValue("Hunt-Destination-Domain")
	}

	// Normalize domain (remove port if present)
	if idx := strings.Index(req.Domain, ":"); idx != -1 {
		req.Domain = req.Domain[:idx]
	}

	// Default context if not specified
	if req.Context == "" {
		req.Context = "default"
	}

	return req, nil
}

// ParseConfigurationRequest parses FreeSWITCH configuration POST parameters
func ParseConfigurationRequest(r *http.Request) (*ConfigurationRequest, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("parse form: %w", err)
	}

	req := &ConfigurationRequest{
		Section:  r.FormValue("section"),
		KeyName:  r.FormValue("key_name"),
		KeyValue: r.FormValue("key_value"),
	}

	return req, nil
}

// ValidateDirectoryRequest validates a directory request
func ValidateDirectoryRequest(req *DirectoryRequest) error {
	if req.Section != "directory" {
		return fmt.Errorf("invalid section: %s", req.Section)
	}

	if req.User == "" {
		return fmt.Errorf("user is required")
	}

	if req.Domain == "" {
		return fmt.Errorf("domain is required")
	}

	return nil
}

// ValidateDialplanRequest validates a dialplan request
func ValidateDialplanRequest(req *DialplanRequest) error {
	if req.Section != "dialplan" {
		return fmt.Errorf("invalid section: %s", req.Section)
	}

	if req.DestinationNumber == "" {
		return fmt.Errorf("destination number is required")
	}

	return nil
}

// ValidateConfigurationRequest validates a configuration request
func ValidateConfigurationRequest(req *ConfigurationRequest) error {
	if req.Section != "configuration" {
		return fmt.Errorf("invalid section: %s", req.Section)
	}

	if req.KeyValue == "" {
		return fmt.Errorf("key value is required")
	}

	return nil
}
