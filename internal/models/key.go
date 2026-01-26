// Package models defines data structures for API keys and audit logs.
package models

import (
	"encoding/json"
	"strings"
	"time"
)

// FlexTime is a time.Time that can parse multiple formats (Python compatibility).
type FlexTime struct {
	time.Time
}

// Common time formats used by Python datetime
var timeFormats = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999999",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05.999999-07:00", // Python datetime with timezone
	"2006-01-02 15:04:05.999999+00:00", // Python datetime UTC explicit
	"2006-01-02 15:04:05.999999",       // Python datetime.now() format
	"2006-01-02 15:04:05-07:00",
	"2006-01-02 15:04:05",
}

// UnmarshalJSON implements json.Unmarshaler for FlexTime.
func (ft *FlexTime) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), "\"")
	if s == "null" || s == "" {
		return nil
	}

	var parsed time.Time
	var err error
	for _, format := range timeFormats {
		parsed, err = time.Parse(format, s)
		if err == nil {
			ft.Time = parsed
			return nil
		}
	}
	return err
}

// MarshalJSON implements json.Marshaler for FlexTime.
func (ft FlexTime) MarshalJSON() ([]byte, error) {
	if ft.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(ft.Format(time.RFC3339Nano))
}

// FlexTimePtr is like FlexTime but for pointer fields.
type FlexTimePtr struct {
	*time.Time
}

// UnmarshalJSON implements json.Unmarshaler for FlexTimePtr.
func (ft *FlexTimePtr) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), "\"")
	if s == "null" || s == "" {
		ft.Time = nil
		return nil
	}

	var parsed time.Time
	var err error
	for _, format := range timeFormats {
		parsed, err = time.Parse(format, s)
		if err == nil {
			ft.Time = &parsed
			return nil
		}
	}
	return err
}

// MarshalJSON implements json.Marshaler for FlexTimePtr.
func (ft FlexTimePtr) MarshalJSON() ([]byte, error) {
	if ft.Time == nil {
		return []byte("null"), nil
	}
	return json.Marshal(ft.Time.Format(time.RFC3339Nano))
}

// APIKey represents an encrypted API key with metadata.
type APIKey struct {
	Name           string      `json:"name"`
	ValueEncrypted string      `json:"value_encrypted"`
	Provider       string      `json:"provider"`
	Description    *string     `json:"description,omitempty"`
	SourceProject  *string     `json:"source_project,omitempty"`
	Tags           []string    `json:"tags,omitempty"`
	CreatedAt      FlexTime    `json:"created_at"`
	UpdatedAt      FlexTime    `json:"updated_at"`
	ExpiresAt      FlexTimePtr `json:"expires_at,omitempty"`
	IsActive       bool        `json:"is_active"`

	// Model information
	ModelVersion      *string  `json:"model_version,omitempty"`
	ModelName         *string  `json:"model_name,omitempty"`
	ModelCapabilities []string `json:"model_capabilities,omitempty"`
}

// NewAPIKey creates a new APIKey with default values.
func NewAPIKey(name, valueEncrypted, provider string) *APIKey {
	now := time.Now()
	return &APIKey{
		Name:              name,
		ValueEncrypted:    valueEncrypted,
		Provider:          provider,
		Tags:              []string{},
		CreatedAt:         FlexTime{now},
		UpdatedAt:         FlexTime{now},
		IsActive:          true,
		ModelCapabilities: []string{},
	}
}

// KeyUsageLog represents an audit log entry with HMAC signature.
type KeyUsageLog struct {
	KeyName   string   `json:"key_name"`
	Project   string   `json:"project"`
	Action    string   `json:"action"` // read, inject, export, add, delete, update
	Timestamp FlexTime `json:"timestamp"`
	Signature *string  `json:"signature,omitempty"`
}

// NewKeyUsageLog creates a new audit log entry.
func NewKeyUsageLog(keyName, project, action string) *KeyUsageLog {
	return &KeyUsageLog{
		KeyName:   keyName,
		Project:   project,
		Action:    action,
		Timestamp: FlexTime{time.Now()},
	}
}

// KeysFile represents the encrypted keys.json structure.
type KeysFile struct {
	Version   string    `json:"version"`
	UpdatedAt string    `json:"updated_at"`
	Keys      []*APIKey `json:"keys"`
}
