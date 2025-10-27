package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/internal/auth"
	"evalgo.org/graphium/models"
)

// UpdateUserRequest represents a user update request
type UpdateUserRequest struct {
	Name    *string        `json:"name,omitempty"`
	Email   *string        `json:"email,omitempty"`
	Enabled *bool          `json:"enabled,omitempty"`
	Roles   *[]models.Role `json:"roles,omitempty"`
}

// ChangePasswordRequest represents a password change request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

// listUsers handles GET /api/v1/users
// @Summary List all users
// @Description Get a list of all users (admin only)
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {array} UserResponse "List of users"
// @Failure 401 {object} APIError "Unauthorized"
// @Failure 403 {object} APIError "Forbidden - Admin access required"
// @Failure 500 {object} APIError "Internal server error"
// @Router /users [get]
func (s *Server) listUsers(c echo.Context) error {
	users, err := s.storage.ListUsers()
	if err != nil {
		return InternalError("Failed to list users", err.Error())
	}

	// Convert to response format
	response := make([]*UserResponse, len(users))
	for i, user := range users {
		response[i] = toUserResponse(user)
	}

	return c.JSON(http.StatusOK, response)
}

// getUser handles GET /api/v1/users/:id
// @Summary Get user by ID
// @Description Get a user by their ID (admin only)
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} UserResponse "User information"
// @Failure 401 {object} APIError "Unauthorized"
// @Failure 403 {object} APIError "Forbidden - Admin access required"
// @Failure 404 {object} APIError "User not found"
// @Failure 500 {object} APIError "Internal server error"
// @Router /users/{id} [get]
func (s *Server) getUser(c echo.Context) error {
	userID := c.Param("id")

	user, err := s.storage.GetUser(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	return c.JSON(http.StatusOK, toUserResponse(user))
}

// updateUser handles PUT /api/v1/users/:id
// @Summary Update user
// @Description Update a user's information (admin only)
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Param user body UpdateUserRequest true "User update data"
// @Success 200 {object} UserResponse "Updated user"
// @Failure 400 {object} APIError "Bad request"
// @Failure 401 {object} APIError "Unauthorized"
// @Failure 403 {object} APIError "Forbidden - Admin access required"
// @Failure 404 {object} APIError "User not found"
// @Failure 500 {object} APIError "Internal server error"
// @Router /users/{id} [put]
func (s *Server) updateUser(c echo.Context) error {
	userID := c.Param("id")

	// Get existing user
	user, err := s.storage.GetUser(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	// Parse update request
	var req UpdateUserRequest
	if err := c.Bind(&req); err != nil {
		return BadRequestError("Invalid request body", err.Error())
	}

	// Apply updates
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Email != nil {
		// Check if email is already in use by another user
		existingUser, err := s.storage.GetUserByEmail(*req.Email)
		if err == nil && existingUser.ID != user.ID {
			return echo.NewHTTPError(http.StatusConflict, "email already in use")
		}
		user.Email = *req.Email
	}
	if req.Enabled != nil {
		user.Enabled = *req.Enabled
	}
	if req.Roles != nil {
		user.Roles = *req.Roles
	}

	// Update timestamp
	user.UpdatedAt = time.Now()

	// Save user
	if err := s.storage.SaveUser(user); err != nil {
		return InternalError("Failed to update user", err.Error())
	}

	// Log update
	if adminID, ok := auth.GetUserID(c); ok {
		if claims, ok := auth.GetClaims(c); ok {
			s.logAuditEvent(c, adminID, claims.Username, "user_updated", "user", true, "")
		}
	}

	return c.JSON(http.StatusOK, toUserResponse(user))
}

// deleteUser handles DELETE /api/v1/users/:id
// @Summary Delete user
// @Description Delete a user (admin only)
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} MessageResponse "Successfully deleted"
// @Failure 401 {object} APIError "Unauthorized"
// @Failure 403 {object} APIError "Forbidden - Admin access required"
// @Failure 404 {object} APIError "User not found"
// @Failure 500 {object} APIError "Internal server error"
// @Router /users/{id} [delete]
func (s *Server) deleteUser(c echo.Context) error {
	userID := c.Param("id")

	// Get user to check if they exist and get rev
	user, err := s.storage.GetUser(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	// Prevent deleting yourself
	if currentUserID, ok := auth.GetUserID(c); ok {
		if currentUserID == userID {
			return echo.NewHTTPError(http.StatusBadRequest, "cannot delete your own account")
		}
	}

	// Delete user
	if err := s.storage.DeleteUser(userID, user.Rev); err != nil {
		return InternalError("Failed to delete user", err.Error())
	}

	// Log deletion
	if adminID, ok := auth.GetUserID(c); ok {
		if claims, ok := auth.GetClaims(c); ok {
			s.logAuditEvent(c, adminID, claims.Username, "user_deleted", "user", true, "")
		}
	}

	return c.JSON(http.StatusOK, MessageResponse{
		Message: fmt.Sprintf("user %s successfully deleted", userID),
	})
}

// changePassword handles POST /api/v1/users/password
// @Summary Change password
// @Description Change current user's password
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param password body ChangePasswordRequest true "Password change data"
// @Success 200 {object} MessageResponse "Password changed successfully"
// @Failure 400 {object} APIError "Bad request"
// @Failure 401 {object} APIError "Unauthorized"
// @Failure 500 {object} APIError "Internal server error"
// @Router /users/password [post]
func (s *Server) changePassword(c echo.Context) error {
	userID, ok := auth.GetUserID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	// Parse request
	var req ChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		return BadRequestError("Invalid request body", err.Error())
	}

	// Get user
	user, err := s.storage.GetUser(userID)
	if err != nil {
		return InternalError("Failed to get user", err.Error())
	}

	// Verify current password
	if err := auth.ComparePassword(req.CurrentPassword, user.PasswordHash); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "current password is incorrect")
	}

	// Hash new password
	newPasswordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return InternalError("Failed to hash password", err.Error())
	}

	// Update password
	user.PasswordHash = newPasswordHash
	user.UpdatedAt = time.Now()

	if err := s.storage.SaveUser(user); err != nil {
		return InternalError("Failed to update password", err.Error())
	}

	// Log password change
	if claims, ok := auth.GetClaims(c); ok {
		s.logAuditEvent(c, userID, claims.Username, "password_changed", "", true, "")
	}

	return c.JSON(http.StatusOK, MessageResponse{
		Message: "password changed successfully",
	})
}

// generateAPIKey handles POST /api/v1/users/api-keys
// @Summary Generate API key
// @Description Generate a new API key for the current user
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string "API key generated"
// @Failure 401 {object} APIError "Unauthorized"
// @Failure 500 {object} APIError "Internal server error"
// @Router /users/api-keys [post]
func (s *Server) generateAPIKey(c echo.Context) error {
	userID, ok := auth.GetUserID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	// Get user
	user, err := s.storage.GetUser(userID)
	if err != nil {
		return InternalError("Failed to get user", err.Error())
	}

	// Generate API key
	apiKey, err := auth.GenerateAPIKey()
	if err != nil {
		return InternalError("Failed to generate API key", err.Error())
	}

	// Hash for storage
	hashedAPIKey, err := auth.HashAPIKey(apiKey)
	if err != nil {
		return InternalError("Failed to hash API key", err.Error())
	}

	// Add to user's API keys
	user.APIKeys = append(user.APIKeys, hashedAPIKey)
	user.UpdatedAt = time.Now()

	if err := s.storage.SaveUser(user); err != nil {
		return InternalError("Failed to save API key", err.Error())
	}

	// Log API key generation
	if claims, ok := auth.GetClaims(c); ok {
		s.logAuditEvent(c, userID, claims.Username, "api_key_generated", "", true, "")
	}

	return c.JSON(http.StatusOK, map[string]string{
		"api_key": apiKey,
		"message": "API key generated successfully. Store this key securely - it will not be shown again.",
	})
}

// revokeAPIKey handles DELETE /api/v1/users/api-keys/:index
// @Summary Revoke API key
// @Description Revoke an API key by its index
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param index path int true "API key index"
// @Success 200 {object} MessageResponse "API key revoked"
// @Failure 400 {object} APIError "Bad request"
// @Failure 401 {object} APIError "Unauthorized"
// @Failure 500 {object} APIError "Internal server error"
// @Router /users/api-keys/{index} [delete]
func (s *Server) revokeAPIKey(c echo.Context) error {
	userID, ok := auth.GetUserID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	// Parse index
	var index int
	if err := echo.PathParamsBinder(c).Int("index", &index).BindError(); err != nil {
		return BadRequestError("Invalid API key index", err.Error())
	}

	// Get user
	user, err := s.storage.GetUser(userID)
	if err != nil {
		return InternalError("Failed to get user", err.Error())
	}

	// Check if index is valid
	if index < 0 || index >= len(user.APIKeys) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid API key index")
	}

	// Remove API key
	user.APIKeys = append(user.APIKeys[:index], user.APIKeys[index+1:]...)
	user.UpdatedAt = time.Now()

	if err := s.storage.SaveUser(user); err != nil {
		return InternalError("Failed to revoke API key", err.Error())
	}

	// Log API key revocation
	if claims, ok := auth.GetClaims(c); ok {
		s.logAuditEvent(c, userID, claims.Username, "api_key_revoked", "", true, "")
	}

	return c.JSON(http.StatusOK, MessageResponse{
		Message: "API key revoked successfully",
	})
}
