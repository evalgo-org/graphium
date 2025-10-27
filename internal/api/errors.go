package api

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

// APIError represents a structured API error with HTTP status code.
type APIError struct {
	Code       int                    `json:"code"`
	Message    string                 `json:"message"`
	Details    string                 `json:"details,omitempty"`
	FieldError map[string]string      `json:"field_errors,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s", e.Message, e.Details)
	}
	return e.Message
}

// NewAPIError creates a new API error.
func NewAPIError(code int, message string, details string) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// Common error constructors
func BadRequestError(message, details string) *APIError {
	return NewAPIError(http.StatusBadRequest, message, details)
}

func NotFoundError(resource, id string) *APIError {
	return &APIError{
		Code:    http.StatusNotFound,
		Message: fmt.Sprintf("%s not found", resource),
		Context: map[string]interface{}{"id": id},
	}
}

func ValidationError(message string, fieldErrors map[string]string) *APIError {
	return &APIError{
		Code:       http.StatusBadRequest,
		Message:    message,
		FieldError: fieldErrors,
	}
}

func InternalError(message, details string) *APIError {
	return NewAPIError(http.StatusInternalServerError, message, details)
}

func ConflictError(message, details string) *APIError {
	return NewAPIError(http.StatusConflict, message, details)
}

// HTTPErrorHandler is a custom error handler for Echo.
func HTTPErrorHandler(err error, c echo.Context) {
	// Don't send response if already sent
	if c.Response().Committed {
		return
	}

	var apiErr *APIError
	code := http.StatusInternalServerError

	// Check if it's an Echo HTTPError
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		apiErr = &APIError{
			Code:    code,
			Message: getHTTPMessage(code),
			Details: fmt.Sprintf("%v", he.Message),
		}
	} else if ae, ok := err.(*APIError); ok {
		// It's already an APIError
		apiErr = ae
		code = ae.Code
	} else {
		// Generic error
		apiErr = &APIError{
			Code:    code,
			Message: "Internal server error",
			Details: err.Error(),
		}
	}

	// Don't expose internal errors in production
	if code == http.StatusInternalServerError && !c.Echo().Debug {
		apiErr.Details = "An internal error occurred. Please try again later."
	}

	// Send JSON response
	if err := c.JSON(code, apiErr); err != nil {
		c.Logger().Error(err)
	}
}

// getHTTPMessage returns a user-friendly message for HTTP status codes.
func getHTTPMessage(code int) string {
	messages := map[int]string{
		http.StatusBadRequest:          "Bad request",
		http.StatusUnauthorized:        "Unauthorized",
		http.StatusForbidden:           "Forbidden",
		http.StatusNotFound:            "Resource not found",
		http.StatusMethodNotAllowed:    "Method not allowed",
		http.StatusConflict:            "Conflict",
		http.StatusUnprocessableEntity: "Unprocessable entity",
		http.StatusTooManyRequests:     "Too many requests",
		http.StatusInternalServerError: "Internal server error",
		http.StatusBadGateway:          "Bad gateway",
		http.StatusServiceUnavailable:  "Service unavailable",
	}

	if msg, ok := messages[code]; ok {
		return msg
	}
	return http.StatusText(code)
}
