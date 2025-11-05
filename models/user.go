// Package models re-exports eve/auth types for graphium.
// This maintains backward compatibility while using the centralized auth package.
package models

import (
	"eve.evalgo.org/auth"
)

// Re-export auth types
type (
	// User is an alias for eve/auth.User
	User = auth.User

	// UserResponse is an alias for eve/auth.UserResponse
	UserResponse = auth.UserResponse

	// RefreshToken is an alias for eve/auth.RefreshToken
	RefreshToken = auth.RefreshToken

	// AuditLog is an alias for eve/auth.AuditLog
	AuditLog = auth.AuditLog

	// CreateUserRequest is an alias for eve/auth.CreateUserRequest
	CreateUserRequest = auth.CreateUserRequest

	// UpdateUserRequest is an alias for eve/auth.UpdateUserRequest
	UpdateUserRequest = auth.UpdateUserRequest

	// TokenPair is an alias for eve/auth.TokenPair
	TokenPair = auth.TokenPair
)

// Re-export role constants
const (
	RoleAdmin  = auth.RoleAdmin
	RoleUser   = auth.RoleUser
	RoleViewer = auth.RoleViewer
	RoleAgent  = auth.RoleAgent
)

// Role is a string alias for role names (for backward compatibility)
// EVE auth uses []string for roles, but we maintain the Role type alias
type Role = string
