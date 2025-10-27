// Package models defines the domain models for Graphium using JSON-LD.
// User represents a user account with authentication and authorization.
package models

import "time"

// Role represents a user role for RBAC
type Role string

const (
	// RoleAdmin has full system access
	RoleAdmin Role = "admin"
	// RoleUser has read-write access to containers and hosts
	RoleUser Role = "user"
	// RoleViewer has read-only access
	RoleViewer Role = "viewer"
	// RoleAgent is for agent authentication
	RoleAgent Role = "agent"
)

// User represents a user account in the system.
// Uses JSON-LD Person type from Schema.org.
type User struct {
	// Context is the JSON-LD @context URL
	Context string `json:"@context,omitempty"`

	// Type is the JSON-LD @type (Person for users)
	Type string `json:"@type,omitempty"`

	// ID is the unique user identifier (maps to CouchDB _id)
	ID string `json:"_id,omitempty"`

	// Rev is the CouchDB document revision
	Rev string `json:"_rev,omitempty"`

	// Username is the unique username for authentication
	Username string `json:"username"`

	// Email is the user's email address
	Email string `json:"email,omitempty"`

	// PasswordHash is the bcrypt hashed password (never sent to client)
	PasswordHash string `json:"password_hash,omitempty"`

	// Name is the user's full name
	Name string `json:"name,omitempty"`

	// Roles are the user's assigned roles for RBAC
	Roles []Role `json:"roles"`

	// Enabled indicates if the user account is active
	Enabled bool `json:"enabled"`

	// APIKeys are the user's API keys for authentication
	APIKeys []string `json:"api_keys,omitempty"`

	// CreatedAt is when the user account was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the user account was last updated
	UpdatedAt time.Time `json:"updated_at"`

	// LastLoginAt is when the user last logged in
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`

	// Metadata contains additional user metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// HasRole checks if the user has a specific role
func (u *User) HasRole(role Role) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// IsAdmin checks if the user has admin role
func (u *User) IsAdmin() bool {
	return u.HasRole(RoleAdmin)
}

// CanWrite checks if the user can write (admin or user role)
func (u *User) CanWrite() bool {
	return u.HasRole(RoleAdmin) || u.HasRole(RoleUser)
}

// CanRead checks if the user can read (any role except disabled)
func (u *User) CanRead() bool {
	return u.Enabled && len(u.Roles) > 0
}

// RefreshToken represents a refresh token for JWT authentication
type RefreshToken struct {
	// Context is the JSON-LD @context URL
	Context string `json:"@context,omitempty"`

	// Type is the JSON-LD @type
	Type string `json:"@type,omitempty"`

	// ID is the unique token identifier (maps to CouchDB _id)
	ID string `json:"_id,omitempty"`

	// Rev is the CouchDB document revision
	Rev string `json:"_rev,omitempty"`

	// UserID is the user this token belongs to
	UserID string `json:"user_id"`

	// Token is the refresh token value (hashed)
	Token string `json:"token"`

	// ExpiresAt is when the token expires
	ExpiresAt time.Time `json:"expires_at"`

	// CreatedAt is when the token was created
	CreatedAt time.Time `json:"created_at"`

	// LastUsedAt is when the token was last used
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`

	// Revoked indicates if the token has been revoked
	Revoked bool `json:"revoked"`
}

// IsValid checks if the refresh token is still valid
func (rt *RefreshToken) IsValid() bool {
	return !rt.Revoked && time.Now().Before(rt.ExpiresAt)
}

// AuditLog represents an audit log entry for security events
type AuditLog struct {
	// Context is the JSON-LD @context URL
	Context string `json:"@context,omitempty"`

	// Type is the JSON-LD @type
	Type string `json:"@type,omitempty"`

	// ID is the unique log entry identifier (maps to CouchDB _id)
	ID string `json:"_id,omitempty"`

	// Rev is the CouchDB document revision
	Rev string `json:"_rev,omitempty"`

	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// UserID is the user who performed the action
	UserID string `json:"user_id,omitempty"`

	// Username is the username (for easier querying)
	Username string `json:"username,omitempty"`

	// Action is the action that was performed
	Action string `json:"action"`

	// Resource is the resource that was affected
	Resource string `json:"resource,omitempty"`

	// ResourceID is the ID of the affected resource
	ResourceID string `json:"resource_id,omitempty"`

	// Method is the HTTP method used
	Method string `json:"method,omitempty"`

	// Path is the API path accessed
	Path string `json:"path,omitempty"`

	// IPAddress is the client IP address
	IPAddress string `json:"ip_address,omitempty"`

	// UserAgent is the client user agent
	UserAgent string `json:"user_agent,omitempty"`

	// Success indicates if the action succeeded
	Success bool `json:"success"`

	// ErrorMessage contains error details if action failed
	ErrorMessage string `json:"error_message,omitempty"`

	// Metadata contains additional context
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}
