package api

import (
	"io"
	"net/http"

	"evalgo.org/graphium/internal/validation"
	"github.com/labstack/echo/v4"
)

// validateContainer validates a container JSON-LD document
func (s *Server) validateContainer(c echo.Context) error {
	// Read request body
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Failed to read request body",
		})
	}

	// Create validator
	validator := validation.New()

	// Validate container
	result, err := validator.ValidateContainer(body)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Validation error",
			Details: err.Error(),
		})
	}

	// Return validation result
	if result.Valid {
		return c.JSON(http.StatusOK, result)
	}

	return c.JSON(http.StatusBadRequest, result)
}

// validateHost validates a host JSON-LD document
func (s *Server) validateHost(c echo.Context) error {
	// Read request body
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Failed to read request body",
		})
	}

	// Create validator
	validator := validation.New()

	// Validate host
	result, err := validator.ValidateHost(body)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Validation error",
			Details: err.Error(),
		})
	}

	// Return validation result
	if result.Valid {
		return c.JSON(http.StatusOK, result)
	}

	return c.JSON(http.StatusBadRequest, result)
}

// validateGeneric validates a generic JSON-LD document based on type
func (s *Server) validateGeneric(c echo.Context) error {
	entityType := c.Param("type")

	// Read request body
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Failed to read request body",
		})
	}

	// Create validator
	validator := validation.New()

	// Validate based on type
	var result *validation.ValidationResult
	switch entityType {
	case "container":
		result, err = validator.ValidateContainer(body)
	case "host":
		result, err = validator.ValidateHost(body)
	default:
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid entity type",
			Details: "Type must be 'container' or 'host'",
		})
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Validation error",
			Details: err.Error(),
		})
	}

	// Return validation result
	if result.Valid {
		return c.JSON(http.StatusOK, result)
	}

	return c.JSON(http.StatusBadRequest, result)
}
