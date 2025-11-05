package web

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/internal/auth"
	"evalgo.org/graphium/models"
)

// ListUsers renders the user list page (admin only)
func (h *Handler) ListUsers(c echo.Context) error {
	claims, ok := auth.GetClaims(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	currentUser, err := h.storage.GetUser(claims.UserID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	// Get all users
	users, err := h.storage.ListUsers()
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load users")
	}

	return Render(c, UsersList(users, currentUser))
}

// ViewUser renders the user detail page
func (h *Handler) ViewUser(c echo.Context) error {
	claims, ok := auth.GetClaims(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	currentUser, err := h.storage.GetUser(claims.UserID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	userID := c.Param("id")
	user, err := h.storage.GetUser(userID)
	if err != nil {
		return c.String(http.StatusNotFound, "User not found")
	}

	isAdmin := currentUser.IsAdmin()

	// Non-admin users can only view their own profile
	if !isAdmin && user.ID != currentUser.ID {
		return c.String(http.StatusForbidden, "Access denied")
	}

	return Render(c, UserDetail(user, currentUser, isAdmin))
}

// NewUserForm renders the create user form (admin only)
func (h *Handler) NewUserForm(c echo.Context) error {
	claims, ok := auth.GetClaims(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	currentUser, err := h.storage.GetUser(claims.UserID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	newUser := &models.User{
		Enabled: true,
		Roles:   []models.Role{models.RoleViewer},
	}

	error := c.QueryParam("error")
	return Render(c, UserFormCreate(newUser, currentUser, error))
}

// CreateUser handles user creation form submission (admin only)
func (h *Handler) CreateUser(c echo.Context) error {
	_, ok := auth.GetClaims(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	username := c.FormValue("username")
	email := c.FormValue("email")
	name := c.FormValue("name")
	password := c.FormValue("password")
	enabled := c.FormValue("enabled") == "on"

	// Parse roles
	roles := []models.Role{}
	if roleValues := c.Request().Form["roles"]; len(roleValues) > 0 {
		for _, roleStr := range roleValues {
			roles = append(roles, models.Role(roleStr))
		}
	}

	// Validate required fields
	if username == "" {
		return c.Redirect(http.StatusFound, "/web/users/new?error=Username+is+required")
	}

	if password == "" {
		return c.Redirect(http.StatusFound, "/web/users/new?error=Password+is+required")
	}

	if len(password) < 8 {
		return c.Redirect(http.StatusFound, "/web/users/new?error=Password+must+be+at+least+8+characters")
	}

	if len(roles) == 0 {
		return c.Redirect(http.StatusFound, "/web/users/new?error=At+least+one+role+is+required")
	}

	// Check if username already exists
	existingUser, err := h.storage.GetUserByUsername(username)
	if err == nil && existingUser != nil {
		return c.Redirect(http.StatusFound, "/web/users/new?error=Username+already+exists")
	}

	// Hash password
	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		return c.Redirect(http.StatusFound, "/web/users/new?error=Failed+to+hash+password")
	}

	// Create user
	user := &models.User{
		Context:      "https://schema.org",
		Type:         "Person",
		Username:     username,
		Email:        email,
		Name:         name,
		PasswordHash: hashedPassword,
		Roles:        roles,
		Enabled:      enabled,
		APIKeys:      []string{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := h.storage.SaveUser(user); err != nil {
		return c.Redirect(http.StatusFound, "/web/users/new?error=Failed+to+create+user")
	}

	// Log audit event (if audit logging is implemented in the future)
	// currentUser, _ := h.storage.GetUser(claims.UserID)
	// if currentUser != nil {
	// 	h.storage.LogAuditEvent(&models.AuditLog{...})
	// }

	return c.Redirect(http.StatusFound, fmt.Sprintf("/web/users/%s", user.ID))
}

// EditUserForm renders the edit user form
func (h *Handler) EditUserForm(c echo.Context) error {
	claims, ok := auth.GetClaims(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	currentUser, err := h.storage.GetUser(claims.UserID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	userID := c.Param("id")
	user, err := h.storage.GetUser(userID)
	if err != nil {
		return c.String(http.StatusNotFound, "User not found")
	}

	// Only admins can edit users (or users can edit themselves)
	isAdmin := currentUser.IsAdmin()
	if !isAdmin && user.ID != currentUser.ID {
		return c.String(http.StatusForbidden, "Access denied")
	}

	error := c.QueryParam("error")
	return Render(c, UserFormEdit(user, currentUser, error))
}

// UpdateUser handles user update form submission
func (h *Handler) UpdateUser(c echo.Context) error {
	claims, ok := auth.GetClaims(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	currentUser, err := h.storage.GetUser(claims.UserID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	userID := c.Param("id")
	user, err := h.storage.GetUser(userID)
	if err != nil {
		return c.String(http.StatusNotFound, "User not found")
	}

	// Only admins can edit users (or users can edit themselves, limited)
	isAdmin := currentUser.IsAdmin()
	if !isAdmin && user.ID != currentUser.ID {
		return c.String(http.StatusForbidden, "Access denied")
	}

	email := c.FormValue("email")
	name := c.FormValue("name")

	// Update basic fields
	user.Email = email
	user.Name = name

	// Only admins can update roles and enabled status
	if isAdmin {
		enabled := c.FormValue("enabled") == "on"
		user.Enabled = enabled

		// Parse roles
		roles := []models.Role{}
		if roleValues := c.Request().Form["roles"]; len(roleValues) > 0 {
			for _, roleStr := range roleValues {
				roles = append(roles, models.Role(roleStr))
			}
		}

		if len(roles) == 0 {
			return c.Redirect(http.StatusFound, fmt.Sprintf("/web/users/%s/edit?error=At+least+one+role+is+required", user.ID))
		}

		user.Roles = roles
	}

	user.UpdatedAt = time.Now()

	if err := h.storage.SaveUser(user); err != nil {
		return c.Redirect(http.StatusFound, fmt.Sprintf("/web/users/%s/edit?error=Failed+to+update+user", user.ID))
	}

	// Log audit event (if audit logging is implemented in the future)
	// h.storage.LogAuditEvent(&models.AuditLog{...})

	return c.Redirect(http.StatusFound, fmt.Sprintf("/web/users/%s", user.ID))
}

// DeleteUser handles user deletion (admin only)
func (h *Handler) DeleteUser(c echo.Context) error {
	claims, ok := auth.GetClaims(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	currentUser, err := h.storage.GetUser(claims.UserID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	userID := c.Param("id")
	user, err := h.storage.GetUser(userID)
	if err != nil {
		return c.String(http.StatusNotFound, "User not found")
	}

	// Cannot delete yourself
	if user.ID == currentUser.ID {
		return c.Redirect(http.StatusFound, "/web/users?error=Cannot+delete+yourself")
	}

	if err := h.storage.DeleteUser(userID); err != nil {
		return c.Redirect(http.StatusFound, "/web/users?error=Failed+to+delete+user")
	}

	// Log audit event (if audit logging is implemented in the future)
	// h.storage.LogAuditEvent(&models.AuditLog{...})

	return c.Redirect(http.StatusFound, "/web/users")
}

// GenerateAPIKey generates a new API key for the user
func (h *Handler) GenerateAPIKey(c echo.Context) error {
	claims, ok := auth.GetClaims(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	userID := c.Param("id")
	user, err := h.storage.GetUser(userID)
	if err != nil {
		return c.String(http.StatusNotFound, "User not found")
	}

	// Users can only generate keys for themselves
	if user.ID != claims.UserID {
		return c.String(http.StatusForbidden, "Access denied")
	}

	// Generate new API key
	apiKey, err := auth.GenerateAPIKey()
	if err != nil {
		return c.Redirect(http.StatusFound, fmt.Sprintf("/web/users/%s?error=Failed+to+generate+API+key", user.ID))
	}

	// Append to user's API keys
	user.APIKeys = append(user.APIKeys, apiKey)
	user.UpdatedAt = time.Now()

	if err := h.storage.SaveUser(user); err != nil {
		return c.Redirect(http.StatusFound, fmt.Sprintf("/web/users/%s?error=Failed+to+save+API+key", user.ID))
	}

	// Log audit event (if audit logging is implemented in the future)
	// h.storage.LogAuditEvent(&models.AuditLog{...})

	return c.Redirect(http.StatusFound, fmt.Sprintf("/web/users/%s", user.ID))
}

// RevokeAPIKey revokes an API key
func (h *Handler) RevokeAPIKey(c echo.Context) error {
	claims, ok := auth.GetClaims(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	userID := c.Param("id")
	user, err := h.storage.GetUser(userID)
	if err != nil {
		return c.String(http.StatusNotFound, "User not found")
	}

	// Users can only revoke their own keys
	if user.ID != claims.UserID {
		return c.String(http.StatusForbidden, "Access denied")
	}

	indexStr := c.Param("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil || index < 0 || index >= len(user.APIKeys) {
		return c.Redirect(http.StatusFound, fmt.Sprintf("/web/users/%s?error=Invalid+API+key+index", user.ID))
	}

	// Remove API key from slice
	user.APIKeys = append(user.APIKeys[:index], user.APIKeys[index+1:]...)
	user.UpdatedAt = time.Now()

	if err := h.storage.SaveUser(user); err != nil {
		return c.Redirect(http.StatusFound, fmt.Sprintf("/web/users/%s?error=Failed+to+revoke+API+key", user.ID))
	}

	// Log audit event (if audit logging is implemented in the future)
	// h.storage.LogAuditEvent(&models.AuditLog{...})

	return c.Redirect(http.StatusFound, fmt.Sprintf("/web/users/%s", user.ID))
}
