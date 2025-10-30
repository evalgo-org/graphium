// Package auth provides authentication and authorization services for Graphium.
// It implements JWT-based authentication with role-based access control (RBAC).
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"evalgo.org/graphium/internal/config"
	"evalgo.org/graphium/models"
)

var (
	// ErrInvalidToken is returned when a JWT token is invalid
	ErrInvalidToken = errors.New("invalid token")
	// ErrExpiredToken is returned when a JWT token has expired
	ErrExpiredToken = errors.New("token has expired")
	// ErrInvalidCredentials is returned when credentials are incorrect
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrUserDisabled is returned when a user account is disabled
	ErrUserDisabled = errors.New("user account is disabled")
)

// Claims represents JWT custom claims
type Claims struct {
	UserID   string        `json:"user_id"`
	Username string        `json:"username"`
	Roles    []models.Role `json:"roles"`
	jwt.RegisteredClaims
}

// TokenPair represents an access token and refresh token
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"` // "Bearer"
}

// JWTService provides JWT authentication services
type JWTService struct {
	secret                 []byte
	expiration             time.Duration
	refreshTokenExpiration time.Duration
}

// NewJWTService creates a new JWT service
func NewJWTService(cfg *config.Config) *JWTService {
	return &JWTService{
		secret:                 []byte(cfg.Security.JWTSecret),
		expiration:             cfg.Security.JWTExpiration,
		refreshTokenExpiration: cfg.Security.RefreshTokenExpiration,
	}
}

// GenerateAgentToken generates a JWT token for agent authentication
// This token uses the agent secret and includes the agent role
func GenerateAgentToken(agentSecret string, hostID string, expiration time.Duration) (string, error) {
	if agentSecret == "" {
		return "", fmt.Errorf("agent secret is required")
	}

	now := time.Now()
	expiresAt := now.Add(expiration)

	claims := Claims{
		UserID:   "agent:" + hostID,
		Username: "agent-" + hostID,
		Roles:    []models.Role{models.RoleAgent},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "graphium-agent",
			Subject:   hostID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(agentSecret))
}

// GenerateToken generates a new JWT access token for a user
func (s *JWTService) GenerateToken(user *models.User) (string, error) {
	if !user.Enabled {
		return "", ErrUserDisabled
	}

	now := time.Now()
	expiresAt := now.Add(s.expiration)

	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		Roles:    user.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "graphium",
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
// It tries to validate with the primary secret, and if that fails and an agent_token_secret
// is configured, it will try that as well (for backward compatibility)
func (s *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// GenerateRefreshToken generates a random refresh token
func (s *JWTService) GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// HashRefreshToken hashes a refresh token for storage
func (s *JWTService) HashRefreshToken(token string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash refresh token: %w", err)
	}
	return string(hash), nil
}

// CompareRefreshToken compares a refresh token with its hash
func (s *JWTService) CompareRefreshToken(token, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(token))
}

// GenerateTokenPair generates both access and refresh tokens
func (s *JWTService) GenerateTokenPair(user *models.User) (*TokenPair, string, error) {
	accessToken, err := s.GenerateToken(user)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.GenerateRefreshToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	expiresAt := time.Now().Add(s.expiration)

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		TokenType:    "Bearer",
	}, refreshToken, nil
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// ComparePassword compares a password with its hash
func ComparePassword(password, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrInvalidCredentials
		}
		return err
	}
	return nil
}

// GenerateAPIKey generates a random API key
func GenerateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate API key: %w", err)
	}
	return "gk_" + base64.URLEncoding.EncodeToString(b), nil
}

// HashAPIKey hashes an API key for storage
func HashAPIKey(key string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash API key: %w", err)
	}
	return string(hash), nil
}

// CompareAPIKey compares an API key with its hash
func CompareAPIKey(key, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(key))
}
