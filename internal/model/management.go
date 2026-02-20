package model

import (
	"encoding/json"
	"time"
)

// Organization represents an organization entity.
type Organization struct {
	OrganizationID string         `json:"organization_id"`
	Name           string         `json:"name"`
	MaxBudget      *float64       `json:"max_budget,omitempty"`
	Spend          float64        `json:"spend"`
	Models         []string       `json:"models,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// Credential represents an encrypted credential.
type Credential struct {
	CredentialID     string    `json:"credential_id"`
	CredentialName   string    `json:"credential_name"`
	Provider         string    `json:"provider"`
	CredentialValues []byte    `json:"credential_values,omitempty"` // NaCl SecretBox encrypted
	CreatedBy        string    `json:"created_by"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// AccessGroup represents a model access group.
type AccessGroup struct {
	GroupID   string    `json:"group_id"`
	Name      string    `json:"name"`
	Models    []string  `json:"models"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Tag represents a spend attribution tag.
type Tag struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// EndUser represents an end user / customer for tracking and budget control.
type EndUser struct {
	ID                 string          `json:"id"`
	EndUserID          string          `json:"end_user_id"`
	Alias              *string         `json:"alias,omitempty"`
	AllowedModelRegion *string         `json:"allowed_model_region,omitempty"`
	DefaultModel       *string         `json:"default_model,omitempty"`
	Budget             *float64        `json:"budget,omitempty"`
	Blocked            bool            `json:"blocked"`
	Metadata           json.RawMessage `json:"metadata,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// GuardrailConfig represents an API-managed guardrail configuration.
type GuardrailConfig struct {
	ID            string          `json:"id"`
	GuardrailName string          `json:"guardrail_name"`
	GuardrailType string          `json:"guardrail_type"`
	Config        json.RawMessage `json:"config"`
	FailurePolicy string          `json:"failure_policy"` // fail_open or fail_closed
	Enabled       bool            `json:"enabled"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// PromptTemplate represents a versioned prompt template.
type PromptTemplate struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Version   int             `json:"version"`
	Template  string          `json:"template"`
	Variables []string        `json:"variables"`
	Model     *string         `json:"model,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// SpendArchive tracks batches of spend logs exported to cold storage.
type SpendArchive struct {
	ID              string    `json:"id"`
	DateFrom        time.Time `json:"date_from"`
	DateTo          time.Time `json:"date_to"`
	StorageType     string    `json:"storage_type"` // s3, gcs
	StorageLocation string    `json:"storage_location"`
	EntryCount      int64     `json:"entry_count"`
	ExportedAt      time.Time `json:"exported_at"`
}

// IPWhitelistEntry represents an allowed IP address or CIDR range.
type IPWhitelistEntry struct {
	ID          string    `json:"id"`
	IPAddress   string    `json:"ip_address"`
	Description *string   `json:"description,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}
