// Package storage provides user storage operations for authentication.
package storage

import (
	"fmt"
	"time"

	"evalgo.org/graphium/models"
	"eve.evalgo.org/db"
)

// SaveUser saves a user to the database
func (s *Storage) SaveUser(user *models.User) error {
	// Set default values
	if user.Context == "" {
		user.Context = "https://schema.org"
	}
	if user.Type == "" {
		user.Type = "Person"
	}

	_, err := s.service.SaveGenericDocument(user)
	return err
}

// GetUser retrieves a user by ID
func (s *Storage) GetUser(id string) (*models.User, error) {
	var user models.User
	err := s.service.GetGenericDocument(id, &user)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (s *Storage) GetUserByUsername(username string) (*models.User, error) {
	query := db.NewQueryBuilder().
		Where("@type", "$eq", "Person").
		And().
		Where("username", "$eq", username).
		Limit(1).
		Build()

	users, err := db.FindTyped[models.User](s.service, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	return &users[0], nil
}

// GetUserByEmail retrieves a user by email
func (s *Storage) GetUserByEmail(email string) (*models.User, error) {
	query := db.NewQueryBuilder().
		Where("@type", "$eq", "Person").
		And().
		Where("email", "$eq", email).
		Limit(1).
		Build()

	users, err := db.FindTyped[models.User](s.service, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	return &users[0], nil
}

// ListUsers retrieves all users
func (s *Storage) ListUsers() ([]*models.User, error) {
	query := db.NewQueryBuilder().
		Where("@type", "$eq", "Person").
		Build()

	users, err := db.FindTyped[models.User](s.service, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	// Convert to pointer slice
	result := make([]*models.User, len(users))
	for i := range users {
		result[i] = &users[i]
	}

	return result, nil
}

// DeleteUser deletes a user by ID
func (s *Storage) DeleteUser(id, rev string) error {
	return s.service.DeleteDocument(id, rev)
}

// SaveRefreshToken saves a refresh token to the database
func (s *Storage) SaveRefreshToken(token *models.RefreshToken) error {
	if token.Context == "" {
		token.Context = "https://schema.org"
	}
	if token.Type == "" {
		token.Type = "RefreshToken"
	}

	_, err := s.service.SaveGenericDocument(token)
	return err
}

// GetRefreshToken retrieves a refresh token by ID
func (s *Storage) GetRefreshToken(id string) (*models.RefreshToken, error) {
	var token models.RefreshToken
	err := s.service.GetGenericDocument(id, &token)
	if err != nil {
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}
	return &token, nil
}

// RevokeRefreshToken revokes a refresh token
func (s *Storage) RevokeRefreshToken(id string) error {
	token, err := s.GetRefreshToken(id)
	if err != nil {
		return err
	}

	token.Revoked = true
	return s.SaveRefreshToken(token)
}

// SaveAuditLog saves an audit log entry
func (s *Storage) SaveAuditLog(log *models.AuditLog) error {
	if log.ID == "" {
		log.ID = fmt.Sprintf("audit-%d", time.Now().UnixNano())
	}
	if log.Context == "" {
		log.Context = "https://schema.org"
	}
	if log.Type == "" {
		log.Type = "AuditLog"
	}

	_, err := s.service.SaveGenericDocument(log)
	return err
}
