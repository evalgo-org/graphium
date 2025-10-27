package api

import (
	"net/http"
	"testing"
)

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name     string
		apiError *APIError
		want     string
	}{
		{
			name: "error with details",
			apiError: &APIError{
				Code:    400,
				Message: "Bad Request",
				Details: "Invalid JSON format",
			},
			want: "Bad Request: Invalid JSON format",
		},
		{
			name: "error without details",
			apiError: &APIError{
				Code:    404,
				Message: "Not Found",
			},
			want: "Not Found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.apiError.Error(); got != tt.want {
				t.Errorf("APIError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBadRequestError(t *testing.T) {
	err := BadRequestError("Invalid input", "Field 'name' is required")

	if err.Code != http.StatusBadRequest {
		t.Errorf("BadRequestError().Code = %v, want %v", err.Code, http.StatusBadRequest)
	}
	if err.Message != "Invalid input" {
		t.Errorf("BadRequestError().Message = %v, want %v", err.Message, "Invalid input")
	}
	if err.Details != "Field 'name' is required" {
		t.Errorf("BadRequestError().Details = %v, want %v", err.Details, "Field 'name' is required")
	}
}

func TestNotFoundError(t *testing.T) {
	err := NotFoundError("Container", "abc123")

	if err.Code != http.StatusNotFound {
		t.Errorf("NotFoundError().Code = %v, want %v", err.Code, http.StatusNotFound)
	}
	if err.Message != "Container not found" {
		t.Errorf("NotFoundError().Message = %v, want %v", err.Message, "Container not found")
	}
	if err.Context == nil {
		t.Error("NotFoundError().Context is nil, want non-nil")
	}
	if id, ok := err.Context["id"].(string); !ok || id != "abc123" {
		t.Errorf("NotFoundError().Context['id'] = %v, want 'abc123'", id)
	}
}

func TestValidationError(t *testing.T) {
	fieldErrors := map[string]string{
		"name":  "Name is required",
		"email": "Invalid email format",
	}
	err := ValidationError("Validation failed", fieldErrors)

	if err.Code != http.StatusBadRequest {
		t.Errorf("ValidationError().Code = %v, want %v", err.Code, http.StatusBadRequest)
	}
	if err.Message != "Validation failed" {
		t.Errorf("ValidationError().Message = %v, want %v", err.Message, "Validation failed")
	}
	if len(err.FieldError) != 2 {
		t.Errorf("ValidationError().FieldError length = %v, want 2", len(err.FieldError))
	}
	if err.FieldError["name"] != "Name is required" {
		t.Errorf("ValidationError().FieldError['name'] = %v, want 'Name is required'", err.FieldError["name"])
	}
}

func TestInternalError(t *testing.T) {
	err := InternalError("Database connection failed", "Connection timeout")

	if err.Code != http.StatusInternalServerError {
		t.Errorf("InternalError().Code = %v, want %v", err.Code, http.StatusInternalServerError)
	}
	if err.Message != "Database connection failed" {
		t.Errorf("InternalError().Message = %v, want %v", err.Message, "Database connection failed")
	}
	if err.Details != "Connection timeout" {
		t.Errorf("InternalError().Details = %v, want %v", err.Details, "Connection timeout")
	}
}

func TestConflictError(t *testing.T) {
	err := ConflictError("Resource conflict", "Resource already exists")

	if err.Code != http.StatusConflict {
		t.Errorf("ConflictError().Code = %v, want %v", err.Code, http.StatusConflict)
	}
	if err.Message != "Resource conflict" {
		t.Errorf("ConflictError().Message = %v, want %v", err.Message, "Resource conflict")
	}
	if err.Details != "Resource already exists" {
		t.Errorf("ConflictError().Details = %v, want %v", err.Details, "Resource already exists")
	}
}

func TestGetHTTPMessage(t *testing.T) {
	tests := []struct {
		name string
		code int
		want string
	}{
		{"Bad Request", http.StatusBadRequest, "Bad request"},
		{"Not Found", http.StatusNotFound, "Resource not found"},
		{"Internal Server Error", http.StatusInternalServerError, "Internal server error"},
		{"Unknown Code", 999, http.StatusText(999)}, // Falls back to http.StatusText for unknown codes
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getHTTPMessage(tt.code); got != tt.want {
				t.Errorf("getHTTPMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}
