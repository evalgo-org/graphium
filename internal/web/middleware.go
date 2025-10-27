package web

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/internal/auth"
	"evalgo.org/graphium/models"
)

// WebAuthMiddleware checks for authentication via cookies
// This is different from API auth middleware which uses Bearer tokens
func (h *Handler) WebAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Skip if auth is disabled
		if !h.config.Security.AuthEnabled {
			return next(c)
		}

		// Get access token from cookie
		cookie, err := c.Cookie("access_token")
		if err != nil || cookie == nil {
			// No access token, redirect to login
			return c.Redirect(http.StatusFound, "/web/auth/login")
		}

		// Validate token
		jwtService := auth.NewJWTService(h.config)
		claims, err := jwtService.ValidateToken(cookie.Value)
		if err != nil {
			// Token invalid or expired, try to refresh
			return h.tryRefreshToken(c, next)
		}

		// Store claims in context
		c.Set(auth.ContextKeyClaims, claims)

		return next(c)
	}
}

// tryRefreshToken attempts to refresh the access token using the refresh token
func (h *Handler) tryRefreshToken(c echo.Context, next echo.HandlerFunc) error {
	// Get refresh token from cookie
	refreshCookie, err := c.Cookie("refresh_token")
	if err != nil || refreshCookie == nil {
		// No refresh token, redirect to login
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	// Validate refresh token
	jwtService := auth.NewJWTService(h.config)
	claims, err := jwtService.ValidateToken(refreshCookie.Value)
	if err != nil {
		// Refresh token invalid or expired, redirect to login
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	// Check if refresh token is in database and not revoked
	tokenHash, err := jwtService.HashRefreshToken(refreshCookie.Value)
	if err != nil {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	refreshToken, err := h.storage.GetRefreshToken(tokenHash)
	if err != nil || refreshToken.Revoked || !refreshToken.IsValid() {
		// Refresh token not found, revoked, or expired
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	// Get user to generate new access token
	user, err := h.storage.GetUser(claims.UserID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	// Generate new access token
	accessToken, err := jwtService.GenerateToken(user)
	if err != nil {
		return c.Redirect(http.StatusFound, "/web/auth/login")
	}

	// Set new access token cookie
	c.SetCookie(&http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		MaxAge:   int(h.config.Security.JWTExpiration.Seconds()),
		HttpOnly: true,
		Secure:   h.config.Server.TLSEnabled,
		SameSite: http.SameSiteLaxMode,
	})

	// Store claims in context
	c.Set(auth.ContextKeyClaims, claims)

	return next(c)
}

// WebAdminMiddleware ensures the user has admin role
func (h *Handler) WebAdminMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		claims, ok := auth.GetClaims(c)
		if !ok {
			return c.Redirect(http.StatusFound, "/web/auth/login")
		}

		// Check if user has admin role
		hasAdmin := false
		for _, role := range claims.Roles {
			if role == models.RoleAdmin {
				hasAdmin = true
				break
			}
		}

		if !hasAdmin {
			return c.String(http.StatusForbidden, "Admin access required")
		}

		return next(c)
	}
}

// WebOptionalAuthMiddleware tries to authenticate but doesn't redirect if not authenticated
// This is useful for pages that show different content based on auth status
func (h *Handler) WebOptionalAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Skip if auth is disabled
		if !h.config.Security.AuthEnabled {
			return next(c)
		}

		// Get access token from cookie
		cookie, err := c.Cookie("access_token")
		if err != nil || cookie == nil {
			// No access token, continue without auth
			return next(c)
		}

		// Validate token
		jwtService := auth.NewJWTService(h.config)
		claims, err := jwtService.ValidateToken(cookie.Value)
		if err == nil {
			// Token valid, store claims in context
			c.Set(auth.ContextKeyClaims, claims)
		}

		return next(c)
	}
}
