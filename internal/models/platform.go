package models

import "time"

// Platform represents an AI platform configuration.
type Platform struct {
	ID                    string     `json:"id"`
	Name                  string     `json:"name"`
	Category              string     `json:"category"` // international, domestic, aggregator, opensource
	APIBase               string     `json:"api_base"`
	APIFormat             string     `json:"api_format"` // openai, claude, google, etc.
	SupportedModels       []string   `json:"supported_models,omitempty"`
	DeprecatedModels      []string   `json:"deprecated_models,omitempty"`
	IsActive              bool       `json:"is_active"`
	RequiresVPN           bool       `json:"requires_vpn"`
	DocsURL               *string    `json:"docs_url,omitempty"`
	PricingURL            *string    `json:"pricing_url,omitempty"`
	SupportsStreaming     bool       `json:"supports_streaming"`
	SupportsFunctionCalls bool       `json:"supports_function_calling"`
	SupportsVision        bool       `json:"supports_vision"`
	SupportsAudio         bool       `json:"supports_audio"`
	LastVerified          *time.Time `json:"last_verified,omitempty"`
}

// PlatformsFile represents the platforms.json structure.
type PlatformsFile struct {
	Version   string     `json:"version"`
	UpdatedAt string     `json:"updated_at"`
	Platforms []Platform `json:"platforms"`
}
