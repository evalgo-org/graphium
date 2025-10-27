package web

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/internal/auth"
	"evalgo.org/graphium/models"
)

// LoginPage renders the login page
func (h *Handler) LoginPage(c echo.Context) error {
	// If user is already logged in, redirect to dashboard
	if claims, ok := auth.GetClaims(c); ok && claims != nil {
		return c.Redirect(http.StatusFound, "/")
	}

	error := c.QueryParam("error")
	return Render(c, LoginPage(error))
}

// Login handles login form submission
func (h *Handler) Login(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	if username == "" || password == "" {
		return c.Redirect(http.StatusFound, "/web/auth/login?error=Username+and+password+are+required")
	}

	// Call the API login handler
	// Get user from storage
	user, err := h.storage.GetUserByUsername(username)
	if err != nil {
		return c.Redirect(http.StatusFound, "/web/auth/login?error=Invalid+username+or+password")
	}

	// Verify password
	if err := auth.ComparePassword(password, user.PasswordHash); err != nil {
		return c.Redirect(http.StatusFound, "/web/auth/login?error=Invalid+username+or+password")
	}

	// Check if user is enabled
	if !user.Enabled {
		return c.Redirect(http.StatusFound, "/web/auth/login?error=Account+is+disabled")
	}

	// Generate JWT tokens
	jwtService := auth.NewJWTService(h.config)
	tokenPair, refreshToken, err := jwtService.GenerateTokenPair(user)
	if err != nil {
		return c.Redirect(http.StatusFound, "/web/auth/login?error=Failed+to+generate+token")
	}

	// Hash and save refresh token to database
	hashedRefreshToken, err := jwtService.HashRefreshToken(refreshToken)
	if err != nil {
		return c.Redirect(http.StatusFound, "/web/auth/login?error=Failed+to+hash+refresh+token")
	}

	refreshTokenModel := &models.RefreshToken{
		Context:   "https://schema.org",
		Type:      "RefreshToken",
		UserID:    user.ID,
		Token:     hashedRefreshToken,
		ExpiresAt: time.Now().Add(h.config.Security.RefreshTokenExpiration),
		CreatedAt: time.Now(),
		Revoked:   false,
	}

	if err := h.storage.SaveRefreshToken(refreshTokenModel); err != nil {
		return c.Redirect(http.StatusFound, "/web/auth/login?error=Failed+to+save+refresh+token")
	}

	// Update last login time
	now := time.Now()
	user.LastLoginAt = &now
	user.UpdatedAt = now
	if err := h.storage.SaveUser(user); err != nil {
		// Log error but don't fail login
	}

	// Set cookies for tokens
	// Access token cookie (short-lived)
	c.SetCookie(&http.Cookie{
		Name:     "access_token",
		Value:    tokenPair.AccessToken,
		Path:     "/",
		MaxAge:   int(h.config.Security.JWTExpiration.Seconds()),
		HttpOnly: true,
		Secure:   h.config.Server.TLSEnabled,
		SameSite: http.SameSiteLaxMode,
	})

	// Refresh token cookie (long-lived)
	c.SetCookie(&http.Cookie{
		Name:     "refresh_token",
		Value:    tokenPair.RefreshToken,
		Path:     "/",
		MaxAge:   int(h.config.Security.RefreshTokenExpiration.Seconds()),
		HttpOnly: true,
		Secure:   h.config.Server.TLSEnabled,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to dashboard
	return c.Redirect(http.StatusFound, "/")
}

// Logout handles logout
func (h *Handler) Logout(c echo.Context) error {
	// Get refresh token from cookie
	refreshTokenCookie, err := c.Cookie("refresh_token")
	if err == nil && refreshTokenCookie != nil {
		// Revoke refresh token in database
		jwtService := auth.NewJWTService(h.config)
		tokenHash, err := jwtService.HashRefreshToken(refreshTokenCookie.Value)
		if err == nil {
			if err := h.storage.RevokeRefreshToken(tokenHash); err != nil {
				// Log error but continue with logout
			}
		}
	}

	// Clear cookies
	c.SetCookie(&http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.config.Server.TLSEnabled,
		SameSite: http.SameSiteLaxMode,
	})

	c.SetCookie(&http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.config.Server.TLSEnabled,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to login page
	return c.Redirect(http.StatusFound, "/web/auth/login")
}

// Profile renders the profile page
func (h *Handler) Profile(c echo.Context) error {
	claims, ok := auth.GetClaims(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	user, err := h.storage.GetUser(claims.UserID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	error := c.QueryParam("error")
	success := c.QueryParam("success")
	return Render(c, ProfilePage(user, error, success))
}

// ChangePassword handles password change form submission
func (h *Handler) ChangePassword(c echo.Context) error {
	claims, ok := auth.GetClaims(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	currentPassword := c.FormValue("current_password")
	newPassword := c.FormValue("new_password")
	confirmPassword := c.FormValue("confirm_password")

	if currentPassword == "" || newPassword == "" || confirmPassword == "" {
		return c.Redirect(http.StatusFound, "/web/profile?error=All+fields+are+required")
	}

	if newPassword != confirmPassword {
		return c.Redirect(http.StatusFound, "/web/profile?error=New+passwords+do+not+match")
	}

	if len(newPassword) < 8 {
		return c.Redirect(http.StatusFound, "/web/profile?error=Password+must+be+at+least+8+characters")
	}

	// Get user
	user, err := h.storage.GetUser(claims.UserID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/web/profile?error=User+not+found")
	}

	// Verify current password
	if err := auth.ComparePassword(currentPassword, user.PasswordHash); err != nil {
		return c.Redirect(http.StatusFound, "/web/profile?error=Current+password+is+incorrect")
	}

	// Hash new password
	hashedPassword, err := auth.HashPassword(newPassword)
	if err != nil {
		return c.Redirect(http.StatusFound, "/web/profile?error=Failed+to+hash+password")
	}

	// Update user
	user.PasswordHash = hashedPassword
	user.UpdatedAt = time.Now()
	if err := h.storage.SaveUser(user); err != nil {
		return c.Redirect(http.StatusFound, "/web/profile?error=Failed+to+update+password")
	}

	return c.Redirect(http.StatusFound, "/web/profile?success=Password+changed+successfully")
}
