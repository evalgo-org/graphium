package api

import (
	"strings"

	"github.com/labstack/echo/v4"
)

// ValidateContentType middleware ensures that requests with a body have the correct Content-Type
func ValidateContentType(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		method := c.Request().Method

		// Only check POST, PUT, PATCH requests
		if method == "POST" || method == "PUT" || method == "PATCH" {
			contentType := c.Request().Header.Get("Content-Type")

			// Allow empty body for some requests
			if c.Request().ContentLength == 0 {
				return next(c)
			}

			// Check if Content-Type is application/json
			if !strings.HasPrefix(contentType, "application/json") {
				return BadRequestError(
					"Invalid Content-Type",
					"Content-Type must be 'application/json'. Got: "+contentType,
				)
			}
		}

		return next(c)
	}
}

// ValidateAcceptHeader middleware ensures that clients can accept JSON responses
func ValidateAcceptHeader(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		accept := c.Request().Header.Get("Accept")

		// If no Accept header, assume */*
		if accept == "" {
			return next(c)
		}

		// Check if Accept includes application/json or */*
		if !strings.Contains(accept, "application/json") &&
		   !strings.Contains(accept, "*/*") &&
		   !strings.Contains(accept, "application/*") {
			return BadRequestError(
				"Invalid Accept header",
				"API only returns JSON. Accept header must include 'application/json' or '*/*'. Got: "+accept,
			)
		}

		return next(c)
	}
}

// ValidateJSONLD middleware validates that JSON-LD documents have required @context and @type fields
func ValidateJSONLD(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		method := c.Request().Method

		// Only validate POST and PUT requests
		if method != "POST" && method != "PUT" {
			return next(c)
		}

		// Parse JSON to check for JSON-LD fields
		var data map[string]interface{}
		if err := c.Bind(&data); err != nil {
			// If binding fails, let the handler deal with it
			return next(c)
		}

		// Check for @context (optional - we allow omission for convenience)
		// But if present, it should be valid
		if context, exists := data["@context"]; exists {
			if context == nil || context == "" {
				return BadRequestError(
					"Invalid JSON-LD",
					"@context field cannot be empty if provided",
				)
			}
		}

		// Check for @type (optional - we allow omission for convenience)
		// But if present, it should be valid
		if typeField, exists := data["@type"]; exists {
			if typeField == nil || typeField == "" {
				return BadRequestError(
					"Invalid JSON-LD",
					"@type field cannot be empty if provided",
				)
			}
		}

		// Rebind the data for the next handler
		c.Set("validated_data", data)

		return next(c)
	}
}

// ValidateIDFormat middleware validates that resource IDs follow expected patterns
func ValidateIDFormat(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		id := c.Param("id")

		// If no ID param, skip validation
		if id == "" {
			return next(c)
		}

		// Check for invalid characters
		if strings.Contains(id, " ") {
			return BadRequestError(
				"Invalid ID format",
				"ID cannot contain spaces",
			)
		}

		// Check for minimum length
		if len(id) < 3 {
			return BadRequestError(
				"Invalid ID format",
				"ID must be at least 3 characters long",
			)
		}

		// Check for maximum length
		if len(id) > 256 {
			return BadRequestError(
				"Invalid ID format",
				"ID must not exceed 256 characters",
			)
		}

		return next(c)
	}
}

// ValidateQueryParams middleware validates common query parameters
func ValidateQueryParams(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Validate limit parameter
		if limitStr := c.QueryParam("limit"); limitStr != "" {
			// parsePagination handles validation, but we can add extra checks here if needed
			// For now, just check it's not negative or non-numeric (parsePagination will handle this)
		}

		// Validate offset parameter
		if offsetStr := c.QueryParam("offset"); offsetStr != "" {
			// parsePagination handles validation
		}

		// Validate status parameter if present
		if status := c.QueryParam("status"); status != "" {
			validStatuses := map[string]bool{
				"running": true, "stopped": true, "paused": true,
				"restarting": true, "exited": true, "dead": true,
				"created": true, "removing": true,
			}
			if !validStatuses[status] {
				return BadRequestError(
					"Invalid status parameter",
					"Status must be one of: running, stopped, paused, restarting, exited, dead, created, removing. Got: "+status,
				)
			}
		}

		return next(c)
	}
}

// SecurityHeaders middleware adds security headers to responses
func SecurityHeaders(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Add security headers
		c.Response().Header().Set("X-Content-Type-Options", "nosniff")
		c.Response().Header().Set("X-Frame-Options", "DENY")
		c.Response().Header().Set("X-XSS-Protection", "1; mode=block")
		c.Response().Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		return next(c)
	}
}
