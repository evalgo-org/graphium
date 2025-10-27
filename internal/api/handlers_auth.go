package api

import (
	"fmt"
	"net/http"
	"time"

	"evalgo.org/graphium/internal/auth"
	"evalgo.org/graphium/models"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Username string        `json:"username" validate:"required,min=3,max=50"`
	Password string        `json:"password" validate:"required,min=8"`
	Email    string        `json:"email" validate:"required,email"`
	Name     string        `json:"name"`
	Roles    []models.Role `json:"roles"`
}

// RefreshRequest represents a token refresh request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// LoginResponse represents a successful login response
type LoginResponse struct {
	User         *UserResponse    `json:"user"`
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	ExpiresAt    time.Time        `json:"expires_at"`
	TokenType    string           `json:"token_type"`
}

// UserResponse represents user data returned to client (without sensitive fields)
type UserResponse struct {
	ID          string        `json:"id"`
	Username    string        `json:"username"`
	Email       string        `json:"email"`
	Name        string        `json:"name"`
	Roles       []models.Role `json:"roles"`
	Enabled     bool          `json:"enabled"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	LastLoginAt *time.Time    `json:"last_login_at,omitempty"`
}

// login handles POST /api/v1/auth/login
// @Summary User login
// @Description Authenticate user with username and password, returns JWT tokens
// @Tags Authentication
// @Accept json
// @Produce json
// @Param credentials body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse "Successfully logged in"
// @Failure 400 {object} APIError "Bad request - Invalid credentials format"
// @Failure 401 {object} APIError "Unauthorized - Invalid username or password"
// @Failure 500 {object} APIError "Internal server error"
// @Router /auth/login [post]
func (s *Server) login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return BadRequestError("Invalid request body", err.Error())
	}

	// Get user by username
	user, err := s.storage.GetUserByUsername(req.Username)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid username or password")
	}

	// Check if user is enabled
	if !user.Enabled {
		return echo.NewHTTPError(http.StatusUnauthorized, "user account is disabled")
	}

	// Verify password
	if err := auth.ComparePassword(req.Password, user.PasswordHash); err != nil {
		// Log failed login attempt
		s.logAuditEvent(c, user.ID, user.Username, "login_failed", "", false, "invalid password")
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid username or password")
	}

	// Generate token pair
	jwtService := auth.NewJWTService(s.config)
	tokenPair, refreshToken, err := jwtService.GenerateTokenPair(user)
	if err != nil {
		return InternalError("Failed to generate tokens", err.Error())
	}

	// Hash and save refresh token
	hashedRefreshToken, err := jwtService.HashRefreshToken(refreshToken)
	if err != nil {
		return InternalError("Failed to hash refresh token", err.Error())
	}

	refreshTokenModel := &models.RefreshToken{
		ID:        fmt.Sprintf("refresh-%s", uuid.New().String()),
		UserID:    user.ID,
		Token:     hashedRefreshToken,
		ExpiresAt: time.Now().Add(s.config.Security.RefreshTokenExpiration),
		CreatedAt: time.Now(),
		Revoked:   false,
	}

	if err := s.storage.SaveRefreshToken(refreshTokenModel); err != nil {
		return InternalError("Failed to save refresh token", err.Error())
	}

	// Update last login time
	now := time.Now()
	user.LastLoginAt = &now
	user.UpdatedAt = now
	if err := s.storage.SaveUser(user); err != nil {
		// Log warning but don't fail the login
		fmt.Printf("Warning: failed to update last login time: %v\n", err)
	}

	// Log successful login
	s.logAuditEvent(c, user.ID, user.Username, "login", "", true, "")

	return c.JSON(http.StatusOK, LoginResponse{
		User:         toUserResponse(user),
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
		TokenType:    tokenPair.TokenType,
	})
}

// register handles POST /api/v1/auth/register
// @Summary Register new user
// @Description Register a new user account (admin only)
// @Tags Authentication
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user body RegisterRequest true "User registration data"
// @Success 201 {object} UserResponse "Successfully created user"
// @Failure 400 {object} APIError "Bad request - Invalid data or validation errors"
// @Failure 409 {object} APIError "Conflict - Username or email already exists"
// @Failure 500 {object} APIError "Internal server error"
// @Router /auth/register [post]
func (s *Server) register(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return BadRequestError("Invalid request body", err.Error())
	}

	// Check if username already exists
	if _, err := s.storage.GetUserByUsername(req.Username); err == nil {
		return echo.NewHTTPError(http.StatusConflict, "username already exists")
	}

	// Check if email already exists
	if _, err := s.storage.GetUserByEmail(req.Email); err == nil {
		return echo.NewHTTPError(http.StatusConflict, "email already exists")
	}

	// Hash password
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		return InternalError("Failed to hash password", err.Error())
	}

	// Set default roles if none provided
	roles := req.Roles
	if len(roles) == 0 {
		roles = []models.Role{models.RoleUser}
	}

	// Create user
	user := &models.User{
		Context:      "https://schema.org",
		Type:         "Person",
		ID:           fmt.Sprintf("user-%s", uuid.New().String()),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		Name:         req.Name,
		Roles:        roles,
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.storage.SaveUser(user); err != nil {
		return InternalError("Failed to create user", err.Error())
	}

	// Log user creation
	if userID, ok := auth.GetUserID(c); ok {
		s.logAuditEvent(c, userID, req.Username, "user_created", "user", true, "")
	}

	return c.JSON(http.StatusCreated, toUserResponse(user))
}

// refresh handles POST /api/v1/auth/refresh
// @Summary Refresh access token
// @Description Get a new access token using a refresh token
// @Tags Authentication
// @Accept json
// @Produce json
// @Param refresh body RefreshRequest true "Refresh token"
// @Success 200 {object} LoginResponse "Successfully refreshed token"
// @Failure 400 {object} APIError "Bad request - Invalid refresh token format"
// @Failure 401 {object} APIError "Unauthorized - Invalid or expired refresh token"
// @Failure 500 {object} APIError "Internal server error"
// @Router /auth/refresh [post]
func (s *Server) refresh(c echo.Context) error {
	var req RefreshRequest
	if err := c.Bind(&req); err != nil {
		return BadRequestError("Invalid request body", err.Error())
	}

	// TODO: Implement refresh token validation
	// For now, return not implemented
	return echo.NewHTTPError(http.StatusNotImplemented, "refresh token endpoint not yet implemented")
}

// logout handles POST /api/v1/auth/logout
// @Summary Logout user
// @Description Revoke refresh token and logout user
// @Tags Authentication
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} MessageResponse "Successfully logged out"
// @Failure 401 {object} APIError "Unauthorized"
// @Failure 500 {object} APIError "Internal server error"
// @Router /auth/logout [post]
func (s *Server) logout(c echo.Context) error {
	userID, ok := auth.GetUserID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	// Log logout
	if claims, ok := auth.GetClaims(c); ok {
		s.logAuditEvent(c, userID, claims.Username, "logout", "", true, "")
	}

	return c.JSON(http.StatusOK, MessageResponse{
		Message: "successfully logged out",
	})
}

// me handles GET /api/v1/auth/me
// @Summary Get current user
// @Description Get information about the currently authenticated user
// @Tags Authentication
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserResponse "Current user information"
// @Failure 401 {object} APIError "Unauthorized"
// @Failure 500 {object} APIError "Internal server error"
// @Router /auth/me [get]
func (s *Server) me(c echo.Context) error {
	userID, ok := auth.GetUserID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	user, err := s.storage.GetUser(userID)
	if err != nil {
		return InternalError("Failed to get user", err.Error())
	}

	return c.JSON(http.StatusOK, toUserResponse(user))
}

// toUserResponse converts a User model to UserResponse (removes sensitive fields)
func toUserResponse(user *models.User) *UserResponse {
	return &UserResponse{
		ID:          user.ID,
		Username:    user.Username,
		Email:       user.Email,
		Name:        user.Name,
		Roles:       user.Roles,
		Enabled:     user.Enabled,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		LastLoginAt: user.LastLoginAt,
	}
}

// logAuditEvent logs an authentication/authorization event
func (s *Server) logAuditEvent(c echo.Context, userID, username, action, resource string, success bool, errorMsg string) {
	auditLog := &models.AuditLog{
		Timestamp:    time.Now(),
		UserID:       userID,
		Username:     username,
		Action:       action,
		Resource:     resource,
		Method:       c.Request().Method,
		Path:         c.Request().URL.Path,
		IPAddress:    c.RealIP(),
		UserAgent:    c.Request().UserAgent(),
		Success:      success,
		ErrorMessage: errorMsg,
	}

	if err := s.storage.SaveAuditLog(auditLog); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Warning: failed to save audit log: %v\n", err)
	}
}
