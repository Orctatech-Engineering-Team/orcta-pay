package apps

import (
	"time"

	"github.com/google/uuid"
)

// App is a registered product integration.
type App struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	Product    string     `json:"product"`
	Prefix     string     `json:"api_key_prefix"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedBy  string     `json:"created_by"`
}

// CreateAppRequest is the body for POST /v1/apps.
type CreateAppRequest struct {
	Name    string `json:"name"`
	Product string `json:"product"`
}

// AppResult is returned once with the plaintext key.
type AppResult struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Product   string    `json:"product"`
	APIKey    string    `json:"api_key,omitempty"`
	Prefix    string    `json:"api_key_prefix"`
	CreatedAt time.Time `json:"created_at"`
}
