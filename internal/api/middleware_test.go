package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestValidateContentType(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		contentType string
		body        string
		wantStatus  int
	}{
		{
			name:        "POST with application/json - valid",
			method:      "POST",
			contentType: "application/json",
			body:        `{"test":"data"}`,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "POST with text/plain - invalid",
			method:      "POST",
			contentType: "text/plain",
			body:        "test data",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "GET request - skip validation",
			method:      "GET",
			contentType: "text/html",
			body:        "",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "POST with empty body - valid",
			method:      "POST",
			contentType: "",
			body:        "",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "PUT with application/json - valid",
			method:      "PUT",
			contentType: "application/json; charset=utf-8",
			body:        `{"test":"data"}`,
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(tt.method, "/", strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			handler := ValidateContentType(func(c echo.Context) error {
				return c.String(http.StatusOK, "OK")
			})

			err := handler(c)

			if tt.wantStatus == http.StatusOK {
				if err != nil {
					t.Errorf("ValidateContentType() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Error("ValidateContentType() error = nil, want error")
				}
				if apiErr, ok := err.(*APIError); ok {
					if apiErr.Code != tt.wantStatus {
						t.Errorf("ValidateContentType() status = %v, want %v", apiErr.Code, tt.wantStatus)
					}
				}
			}
		})
	}
}

func TestValidateAcceptHeader(t *testing.T) {
	tests := []struct {
		name       string
		accept     string
		wantStatus int
	}{
		{
			name:       "application/json - valid",
			accept:     "application/json",
			wantStatus: http.StatusOK,
		},
		{
			name:       "*/* - valid",
			accept:     "*/*",
			wantStatus: http.StatusOK,
		},
		{
			name:       "application/* - valid",
			accept:     "application/*",
			wantStatus: http.StatusOK,
		},
		{
			name:       "no accept header - valid",
			accept:     "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "text/html - invalid",
			accept:     "text/html",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "complex accept with json - valid",
			accept:     "text/html,application/json;q=0.9,*/*;q=0.8",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest("GET", "/", nil)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			handler := ValidateAcceptHeader(func(c echo.Context) error {
				return c.String(http.StatusOK, "OK")
			})

			err := handler(c)

			if tt.wantStatus == http.StatusOK {
				if err != nil {
					t.Errorf("ValidateAcceptHeader() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Error("ValidateAcceptHeader() error = nil, want error")
				}
			}
		})
	}
}

func TestValidateIDFormat(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		wantStatus int
	}{
		{
			name:       "valid ID",
			id:         "abc123",
			wantStatus: http.StatusOK,
		},
		{
			name:       "ID too short",
			id:         "ab",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "ID with space - invalid",
			id:         "abc 123",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "ID at min length",
			id:         "abc",
			wantStatus: http.StatusOK,
		},
		{
			name:       "empty ID - skip validation",
			id:         "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "very long ID - exceeds max",
			id:         strings.Repeat("a", 300),
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest("GET", "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetParamNames("id")
			c.SetParamValues(tt.id)

			handler := ValidateIDFormat(func(c echo.Context) error {
				return c.String(http.StatusOK, "OK")
			})

			err := handler(c)

			if tt.wantStatus == http.StatusOK {
				if err != nil {
					t.Errorf("ValidateIDFormat() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Error("ValidateIDFormat() error = nil, want error")
				}
			}
		})
	}
}

func TestValidateQueryParams(t *testing.T) {
	tests := []struct {
		name        string
		queryParams map[string]string
		wantStatus  int
	}{
		{
			name: "valid status - running",
			queryParams: map[string]string{
				"status": "running",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "valid status - stopped",
			queryParams: map[string]string{
				"status": "stopped",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid status",
			queryParams: map[string]string{
				"status": "invalid_status",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "no query params",
			queryParams: map[string]string{},
			wantStatus:  http.StatusOK,
		},
		{
			name: "valid limit and offset",
			queryParams: map[string]string{
				"limit":  "50",
				"offset": "10",
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest("GET", "/", nil)
			q := req.URL.Query()
			for k, v := range tt.queryParams {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			handler := ValidateQueryParams(func(c echo.Context) error {
				return c.String(http.StatusOK, "OK")
			})

			err := handler(c)

			if tt.wantStatus == http.StatusOK {
				if err != nil {
					t.Errorf("ValidateQueryParams() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Error("ValidateQueryParams() error = nil, want error")
				}
			}
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := SecurityHeaders(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	err := handler(c)
	if err != nil {
		t.Fatalf("SecurityHeaders() error = %v, want nil", err)
	}

	// Check security headers
	headers := c.Response().Header()

	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-Xss-Protection":       "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for header, expectedValue := range expectedHeaders {
		gotValue := headers.Get(header)
		if gotValue != expectedValue {
			t.Errorf("SecurityHeaders() %s = %v, want %v", header, gotValue, expectedValue)
		}
	}
}
