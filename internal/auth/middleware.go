// Package auth provides authentication middleware for Graphium API.
package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"

	"evalgo.org/graphium/internal/config"
	"evalgo.org/graphium/models"
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

// RequireAPIKey is middleware that requires a valid API key
func (m *Middleware) RequireAPIKey(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Skip if no API keys are configured
		if len(m.config.Security.APIKeys) == 0 {
			return next(c)
		}

		// Extract API key from Authorization header or X-API-Key header
		apiKey := c.Request().Header.Get("X-API-Key")
		if apiKey == "" {
			// Try Authorization header with "ApiKey" scheme
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && parts[0] == "ApiKey" {
					apiKey = parts[1]
				}
			}
		}

		if apiKey == "" {
			return echo.NewHTTPError(http.StatusUnauthorized, "missing API key")
		}

		// Check if API key is valid
		validKey := false
		for _, key := range m.config.Security.APIKeys {
			if key == apiKey {
				validKey = true
				break
			}
		}

		if !validKey {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid API key")
		}

		return next(c)
	}
}

// RequireAgentAuth is middleware that requires agent authentication
func (m *Middleware) RequireAgentAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Skip if agent authentication is not configured
		if m.config.Security.AgentTokenSecret == "" {
			return next(c)
		}

		// Extract token from Authorization header
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" {
			return echo.NewHTTPError(http.StatusUnauthorized, "missing agent token")
		}

		// Parse Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization header format")
		}

		tokenString := parts[1]

		// Validate agent token (same validation as JWT but with agent secret)
		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			// Verify signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(m.config.Security.AgentTokenSecret), nil
		})

		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				return echo.NewHTTPError(http.StatusUnauthorized, "agent token has expired")
			}
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid agent token")
		}

		claims, ok := token.Claims.(*Claims)
		if !ok || !token.Valid {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid agent token")
		}

		// Verify this is an agent token (has agent role)
		isAgent := false
		for _, role := range claims.Roles {
			if role == models.RoleAgent {
				isAgent = true
				break
			}
		}

		if !isAgent {
			return echo.NewHTTPError(http.StatusForbidden, "token does not have agent permissions")
		}

		// Store claims in context
		c.Set(ContextKeyClaims, claims)

		return next(c)
	}
}
