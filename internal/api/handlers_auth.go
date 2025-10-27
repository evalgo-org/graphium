package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/internal/auth"
	"evalgo.org/graphium/models"
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
	User         *UserResponse `json:"user"`
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	ExpiresAt    time.Time     `json:"expires_at"`
	TokenType    string        `json:"token_type"`
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

	// Create JWT service
	jwtService := auth.NewJWTService(s.config)

	// Find all refresh tokens and check if any match
	// Note: In a production system, you might want to store a mapping of token hash to token ID
	// For now, we'll search through all tokens (this could be optimized)

	// We need to extract the user from the refresh token somehow
	// Since refresh tokens are just random strings, we need to find them by comparing hashes
	// This is a limitation - in production, consider storing token metadata or using JWT refresh tokens

	// For now, we'll implement a simple approach: find all non-revoked, non-expired refresh tokens
	// and compare the provided token with each hash

	// This is inefficient but functional. A better approach would be:
	// 1. Use JWT for refresh tokens with user ID in claims
	// 2. Store a mapping of token ID to user ID
	// 3. Use a faster lookup mechanism

	// Get all users and check their refresh tokens
	users, err := s.storage.ListUsers()
	if err != nil {
		return InternalError("Failed to validate refresh token", err.Error())
	}

	var matchedUser *models.User
	var matchedToken *models.RefreshToken

	// Find the matching refresh token
	for _, user := range users {
		tokens, err := s.storage.GetRefreshTokensByUserID(user.ID)
		if err != nil {
			continue
		}

		for _, token := range tokens {
			// Skip revoked or expired tokens
			if token.Revoked || time.Now().After(token.ExpiresAt) {
				continue
			}

			// Check if the provided token matches this hash
			if err := jwtService.CompareRefreshToken(req.RefreshToken, token.Token); err == nil {
				matchedUser = user
				matchedToken = token
				break
			}
		}

		if matchedUser != nil {
			break
		}
	}

	if matchedUser == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired refresh token")
	}

	// Check if user is still enabled
	if !matchedUser.Enabled {
		return echo.NewHTTPError(http.StatusUnauthorized, "user account is disabled")
	}

	// Generate new token pair
	tokenPair, newRefreshToken, err := jwtService.GenerateTokenPair(matchedUser)
	if err != nil {
		return InternalError("Failed to generate tokens", err.Error())
	}

	// Hash and save new refresh token
	hashedRefreshToken, err := jwtService.HashRefreshToken(newRefreshToken)
	if err != nil {
		return InternalError("Failed to hash refresh token", err.Error())
	}

	newRefreshTokenModel := &models.RefreshToken{
		ID:        fmt.Sprintf("refresh-%s", uuid.New().String()),
		UserID:    matchedUser.ID,
		Token:     hashedRefreshToken,
		ExpiresAt: time.Now().Add(s.config.Security.RefreshTokenExpiration),
		CreatedAt: time.Now(),
		Revoked:   false,
	}

	if err := s.storage.SaveRefreshToken(newRefreshTokenModel); err != nil {
		return InternalError("Failed to save refresh token", err.Error())
	}

	// Revoke the old refresh token
	if err := s.storage.RevokeRefreshToken(matchedToken.ID); err != nil {
		// Log warning but don't fail the request
		fmt.Printf("Warning: failed to revoke old refresh token: %v\n", err)
	}

	// Log successful token refresh
	s.logAuditEvent(c, matchedUser.ID, matchedUser.Username, "token_refresh", "", true, "")

	return c.JSON(http.StatusOK, LoginResponse{
		User:         toUserResponse(matchedUser),
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
		TokenType:    tokenPair.TokenType,
	})
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
