// Package auth provides authentication middleware for Graphium API.
package auth

import (
	"net/http"
	"strings"

	"evalgo.org/graphium/internal/config"
	"evalgo.org/graphium/models"
	"github.com/labstack/echo/v4"
)

const (
	// ContextKeyUser is the key for storing user in context
	ContextKeyUser = "user"
	// ContextKeyClaims is the key for storing JWT claims in context
	ContextKeyClaims = "claims"
)

// Middleware is the authentication middleware
type Middleware struct {
	jwtService *JWTService
	config     *config.Config
}

// NewMiddleware creates a new authentication middleware
func NewMiddleware(cfg *config.Config) *Middleware {
	return &Middleware{
		jwtService: NewJWTService(cfg),
		config:     cfg,
	}
}

// RequireAuth is middleware that requires JWT authentication
func (m *Middleware) RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Skip if auth is disabled
		if !m.config.Security.AuthEnabled {
			return next(c)
		}

		// Extract token from Authorization header
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" {
			return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
		}

		// Parse Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization header format")
		}

		tokenString := parts[1]

		// Validate token
		claims, err := m.jwtService.ValidateToken(tokenString)
		if err != nil {
			if err == ErrExpiredToken {
				return echo.NewHTTPError(http.StatusUnauthorized, "token has expired")
			}
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
		}

		// Store claims in context
		c.Set(ContextKeyClaims, claims)

		return next(c)
	}
}

// RequireRole is middleware that requires a specific role
func (m *Middleware) RequireRole(roles ...models.Role) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip if auth is disabled
			if !m.config.Security.AuthEnabled {
				return next(c)
			}

			// Get claims from context
			claims, ok := c.Get(ContextKeyClaims).(*Claims)
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
			}

			// Check if user has any of the required roles
			hasRole := false
			for _, requiredRole := range roles {
				for _, userRole := range claims.Roles {
					if userRole == requiredRole {
						hasRole = true
						break
					}
				}
				if hasRole {
					break
				}
			}

			if !hasRole {
				return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
			}

			return next(c)
		}
	}
}

// RequireAdmin is middleware that requires admin role
func (m *Middleware) RequireAdmin(next echo.HandlerFunc) echo.HandlerFunc {
	return m.RequireRole(models.RoleAdmin)(next)
}

// RequireWrite is middleware that requires write permissions (admin or user role)
func (m *Middleware) RequireWrite(next echo.HandlerFunc) echo.HandlerFunc {
	return m.RequireRole(models.RoleAdmin, models.RoleUser)(next)
}

// RequireRead is middleware that requires read permissions (any authenticated user)
func (m *Middleware) RequireRead(next echo.HandlerFunc) echo.HandlerFunc {
	return m.RequireAuth(next)
}

// GetClaims extracts JWT claims from Echo context
func GetClaims(c echo.Context) (*Claims, bool) {
	claims, ok := c.Get(ContextKeyClaims).(*Claims)
	return claims, ok
}

// GetUserID extracts user ID from JWT claims in context
func GetUserID(c echo.Context) (string, bool) {
	claims, ok := GetClaims(c)
	if !ok {
		return "", false
	}
	return claims.UserID, true
}

// HasRole checks if the current user has a specific role
func HasRole(c echo.Context, role models.Role) bool {
	claims, ok := GetClaims(c)
	if !ok {
		return false
	}

	for _, r := range claims.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// IsAdmin checks if the current user is an admin
func IsAdmin(c echo.Context) bool {
	return HasRole(c, models.RoleAdmin)
}

// CanWrite checks if the current user can write
func CanWrite(c echo.Context) bool {
	return HasRole(c, models.RoleAdmin) || HasRole(c, models.RoleUser)
}
