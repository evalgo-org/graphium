// Package storage provides user storage operations using eve/auth.
package storage

import (
	"eve.evalgo.org/auth"

	"evalgo.org/graphium/models"
)

// userStore is the eve/auth UserStore instance
var userStore auth.UserStore

// initUserStore initializes the user store with the CouchDB service
func (s *Storage) initUserStore() {
	if userStore == nil {
		userStore = auth.NewCouchDBUserStore(s.service)
	}
}

// SaveUser saves a user to the database using eve/auth storage
func (s *Storage) SaveUser(user *models.User) error {
	s.initUserStore()

	// For new users, use CreateUser
	if user.ID == "" {
		return userStore.CreateUser(user)
	}

	// For existing users, use UpdateUser
	return userStore.UpdateUser(user)
}

// GetUser retrieves a user by ID using eve/auth storage
func (s *Storage) GetUser(id string) (*models.User, error) {
	s.initUserStore()
	return userStore.GetUser(id)
}

// GetUserByUsername retrieves a user by username using eve/auth storage
func (s *Storage) GetUserByUsername(username string) (*models.User, error) {
	s.initUserStore()
	return userStore.GetUserByUsername(username)
}

// GetUserByEmail retrieves a user by email using eve/auth storage
func (s *Storage) GetUserByEmail(email string) (*models.User, error) {
	s.initUserStore()
	return userStore.GetUserByEmail(email)
}

// ListUsers retrieves all users using eve/auth storage
func (s *Storage) ListUsers() ([]*models.User, error) {
	s.initUserStore()
	return userStore.ListUsers()
}

// UpdateUser updates a user using eve/auth storage
func (s *Storage) UpdateUser(user *models.User) error {
	s.initUserStore()
	return userStore.UpdateUser(user)
}

// DeleteUser deletes a user using eve/auth storage
func (s *Storage) DeleteUser(id string) error {
	s.initUserStore()
	return userStore.DeleteUser(id)
}

// RecordLoginAttempt records a login attempt using eve/auth storage
func (s *Storage) RecordLoginAttempt(username string, success bool) error {
	s.initUserStore()
	return userStore.RecordLoginAttempt(username, success)
}

// SaveRefreshToken saves a refresh token using eve/auth storage
func (s *Storage) SaveRefreshToken(token *models.RefreshToken) error {
	s.initUserStore()
	return userStore.SaveRefreshToken(token)
}

// GetRefreshToken retrieves a refresh token by ID using eve/auth storage
func (s *Storage) GetRefreshToken(id string) (*models.RefreshToken, error) {
	s.initUserStore()
	return userStore.GetRefreshToken(id)
}

// GetRefreshTokensByUserID retrieves all refresh tokens for a user using eve/auth storage
func (s *Storage) GetRefreshTokensByUserID(userID string) ([]*models.RefreshToken, error) {
	s.initUserStore()
	return userStore.GetRefreshTokensByUserID(userID)
}

// RevokeRefreshToken revokes a refresh token using eve/auth storage
func (s *Storage) RevokeRefreshToken(id string) error {
	s.initUserStore()
	return userStore.RevokeRefreshToken(id)
}

// DeleteExpiredRefreshTokens deletes expired refresh tokens using eve/auth storage
func (s *Storage) DeleteExpiredRefreshTokens() error {
	s.initUserStore()
	return userStore.DeleteExpiredRefreshTokens()
}

// SaveAuditLog saves an audit log entry using eve/auth storage
func (s *Storage) SaveAuditLog(log *models.AuditLog) error {
	s.initUserStore()
	return userStore.SaveAuditLog(log)
}

// GetAuditLogs retrieves audit logs using eve/auth storage
func (s *Storage) GetAuditLogs(criteria auth.AuditSearchCriteria) ([]*models.AuditLog, error) {
	s.initUserStore()
	return userStore.GetAuditLogs(criteria)
}
