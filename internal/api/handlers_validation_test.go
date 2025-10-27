package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"evalgo.org/graphium/internal/config"
	_ "evalgo.org/graphium/internal/storage"
	"evalgo.org/graphium/internal/validation"
)

func setupTestServer(t *testing.T) (*Server, *echo.Echo) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	// Create a mock server without storage
	e := echo.New()
	server := &Server{
		echo:   e,
		config: cfg,
	}

	return server, e
}

func TestValidateContainer_Valid(t *testing.T) {
	server, e := setupTestServer(t)

	validContainer := `{
		"@context": "https://schema.org",
		"@type": "SoftwareApplication",
		"@id": "test-container",
		"name": "test",
		"executableName": "nginx:latest",
		"status": "running",
		"hostedOn": "host-01"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/validate/container", bytes.NewBufferString(validContainer))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := server.validateContainer(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var result validation.ValidationResult
	err = json.Unmarshal(rec.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.True(t, result.Valid)
}

func TestValidateContainer_Invalid(t *testing.T) {
	server, e := setupTestServer(t)

	invalidContainer := `{
		"@context": "https://schema.org",
		"@type": "SoftwareApplication",
		"@id": "test-container"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/validate/container", bytes.NewBufferString(invalidContainer))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := server.validateContainer(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var result validation.ValidationResult
	err = json.Unmarshal(rec.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
}

func TestValidateHost_Valid(t *testing.T) {
	server, e := setupTestServer(t)

	validHost := `{
		"@context": "https://schema.org",
		"@type": "ComputerSystem",
		"@id": "host-01",
		"name": "web-server-01",
		"ipAddress": "192.168.1.10"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/validate/host", bytes.NewBufferString(validHost))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := server.validateHost(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var result validation.ValidationResult
	err = json.Unmarshal(rec.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.True(t, result.Valid)
}

func TestValidateHost_Invalid(t *testing.T) {
	server, e := setupTestServer(t)

	invalidHost := `{
		"@context": "https://schema.org",
		"@type": "ComputerSystem",
		"@id": "host-01"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/validate/host", bytes.NewBufferString(invalidHost))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := server.validateHost(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var result validation.ValidationResult
	err = json.Unmarshal(rec.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
}

func TestValidateGeneric_Container(t *testing.T) {
	server, e := setupTestServer(t)

	validContainer := `{
		"@context": "https://schema.org",
		"@type": "SoftwareApplication",
		"@id": "test",
		"name": "test",
		"executableName": "nginx:latest",
		"hostedOn": "host-01"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/validate/container", bytes.NewBufferString(validContainer))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("type")
	c.SetParamValues("container")

	err := server.validateGeneric(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestValidateGeneric_Host(t *testing.T) {
	server, e := setupTestServer(t)

	validHost := `{
		"@context": "https://schema.org",
		"@type": "ComputerSystem",
		"@id": "host-01",
		"name": "test-host",
		"ipAddress": "192.168.1.10"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/validate/host", bytes.NewBufferString(validHost))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("type")
	c.SetParamValues("host")

	err := server.validateGeneric(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestValidateGeneric_InvalidType(t *testing.T) {
	server, e := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/validate/invalid", bytes.NewBufferString("{}"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("type")
	c.SetParamValues("invalid")

	err := server.validateGeneric(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
