package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	v := New()
	assert.NotNil(t, v)
	assert.NotNil(t, v.structValidator)
	assert.NotNil(t, v.jsonldProcessor)
}

func TestValidateContainer_Valid(t *testing.T) {
	v := New()

	validContainer := []byte(`{
		"@context": "https://schema.org",
		"@type": "SoftwareApplication",
		"@id": "test-container",
		"name": "test",
		"executableName": "nginx:latest",
		"status": "running",
		"hostedOn": "host-01"
	}`)

	result, err := v.ValidateContainer(validContainer)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestValidateContainer_MissingContext(t *testing.T) {
	v := New()

	invalidContainer := []byte(`{
		"@type": "SoftwareApplication",
		"@id": "test-container",
		"name": "test",
		"executableName": "nginx:latest",
		"hostedOn": "host-01"
	}`)

	result, err := v.ValidateContainer(invalidContainer)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)

	// Check that @context error is present
	hasContextError := false
	for _, e := range result.Errors {
		if e.Field == "@context" {
			hasContextError = true
			break
		}
	}
	assert.True(t, hasContextError, "Should have @context error")
}

func TestValidateContainer_MissingRequiredFields(t *testing.T) {
	v := New()

	tests := []struct {
		name          string
		json          string
		expectedField string
	}{
		{
			name: "missing name",
			json: `{
				"@context": "https://schema.org",
				"@type": "SoftwareApplication",
				"@id": "test",
				"executableName": "nginx:latest",
				"hostedOn": "host-01"
			}`,
			expectedField: "name",
		},
		{
			name: "missing executableName",
			json: `{
				"@context": "https://schema.org",
				"@type": "SoftwareApplication",
				"@id": "test",
				"name": "test",
				"hostedOn": "host-01"
			}`,
			expectedField: "executableName",
		},
		{
			name: "missing hostedOn",
			json: `{
				"@context": "https://schema.org",
				"@type": "SoftwareApplication",
				"@id": "test",
				"name": "test",
				"executableName": "nginx:latest"
			}`,
			expectedField: "hostedOn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.ValidateContainer([]byte(tt.json))
			require.NoError(t, err)
			assert.False(t, result.Valid)

			hasError := false
			for _, e := range result.Errors {
				if e.Field == tt.expectedField {
					hasError = true
					break
				}
			}
			assert.True(t, hasError, "Should have error for field: %s", tt.expectedField)
		})
	}
}

func TestValidateContainer_InvalidStatus(t *testing.T) {
	v := New()

	invalidContainer := []byte(`{
		"@context": "https://schema.org",
		"@type": "SoftwareApplication",
		"@id": "test",
		"name": "test",
		"executableName": "nginx:latest",
		"status": "invalid-status",
		"hostedOn": "host-01"
	}`)

	result, err := v.ValidateContainer(invalidContainer)
	require.NoError(t, err)
	assert.False(t, result.Valid)

	hasStatusError := false
	for _, e := range result.Errors {
		if e.Field == "status" {
			hasStatusError = true
			assert.Equal(t, "invalid-status", e.Value)
			break
		}
	}
	assert.True(t, hasStatusError)
}

func TestValidateContainer_InvalidPorts(t *testing.T) {
	v := New()

	tests := []struct {
		name        string
		json        string
		expectError string
	}{
		{
			name: "invalid host port - too high",
			json: `{
				"@context": "https://schema.org",
				"@type": "SoftwareApplication",
				"@id": "test",
				"name": "test",
				"executableName": "nginx:latest",
				"hostedOn": "host-01",
				"ports": [{"hostPort": 99999, "containerPort": 80, "protocol": "tcp"}]
			}`,
			expectError: "ports[0].hostPort",
		},
		{
			name: "invalid container port - negative",
			json: `{
				"@context": "https://schema.org",
				"@type": "SoftwareApplication",
				"@id": "test",
				"name": "test",
				"executableName": "nginx:latest",
				"hostedOn": "host-01",
				"ports": [{"hostPort": 80, "containerPort": -1, "protocol": "tcp"}]
			}`,
			expectError: "ports[0].containerPort",
		},
		{
			name: "invalid protocol",
			json: `{
				"@context": "https://schema.org",
				"@type": "SoftwareApplication",
				"@id": "test",
				"name": "test",
				"executableName": "nginx:latest",
				"hostedOn": "host-01",
				"ports": [{"hostPort": 80, "containerPort": 80, "protocol": "invalid"}]
			}`,
			expectError: "ports[0].protocol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.ValidateContainer([]byte(tt.json))
			require.NoError(t, err)
			assert.False(t, result.Valid)

			hasError := false
			for _, e := range result.Errors {
				if e.Field == tt.expectError {
					hasError = true
					break
				}
			}
			assert.True(t, hasError, "Should have error for: %s", tt.expectError)
		})
	}
}

func TestValidateHost_Valid(t *testing.T) {
	v := New()

	validHost := []byte(`{
		"@context": "https://schema.org",
		"@type": "ComputerSystem",
		"@id": "host-01",
		"name": "web-server-01",
		"ipAddress": "192.168.1.10",
		"status": "active"
	}`)

	result, err := v.ValidateHost(validHost)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestValidateHost_MissingRequiredFields(t *testing.T) {
	v := New()

	tests := []struct {
		name          string
		json          string
		expectedField string
	}{
		{
			name: "missing name",
			json: `{
				"@context": "https://schema.org",
				"@type": "ComputerSystem",
				"@id": "host-01",
				"ipAddress": "192.168.1.10"
			}`,
			expectedField: "name",
		},
		{
			name: "missing ipAddress",
			json: `{
				"@context": "https://schema.org",
				"@type": "ComputerSystem",
				"@id": "host-01",
				"name": "test-host"
			}`,
			expectedField: "ipAddress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.ValidateHost([]byte(tt.json))
			require.NoError(t, err)
			assert.False(t, result.Valid)

			hasError := false
			for _, e := range result.Errors {
				if e.Field == tt.expectedField {
					hasError = true
					break
				}
			}
			assert.True(t, hasError, "Should have error for field: %s", tt.expectedField)
		})
	}
}

func TestValidateHost_InvalidIPAddress(t *testing.T) {
	v := New()

	invalidHost := []byte(`{
		"@context": "https://schema.org",
		"@type": "ComputerSystem",
		"@id": "host-01",
		"name": "test-host",
		"ipAddress": "999.999.999.999"
	}`)

	result, err := v.ValidateHost(invalidHost)
	require.NoError(t, err)
	assert.False(t, result.Valid)

	hasIPError := false
	for _, e := range result.Errors {
		if e.Field == "ipAddress" {
			hasIPError = true
			break
		}
	}
	assert.True(t, hasIPError)
}

func TestValidateHost_NegativeValues(t *testing.T) {
	v := New()

	tests := []struct {
		name        string
		json        string
		expectError string
	}{
		{
			name: "negative CPU",
			json: `{
				"@context": "https://schema.org",
				"@type": "ComputerSystem",
				"@id": "host-01",
				"name": "test",
				"ipAddress": "192.168.1.10",
				"cpu": -4
			}`,
			expectError: "cpu",
		},
		{
			name: "negative memory",
			json: `{
				"@context": "https://schema.org",
				"@type": "ComputerSystem",
				"@id": "host-01",
				"name": "test",
				"ipAddress": "192.168.1.10",
				"memory": -1000
			}`,
			expectError: "memory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.ValidateHost([]byte(tt.json))
			require.NoError(t, err)
			assert.False(t, result.Valid)

			hasError := false
			for _, e := range result.Errors {
				if e.Field == tt.expectError {
					hasError = true
					break
				}
			}
			assert.True(t, hasError, "Should have error for: %s", tt.expectError)
		})
	}
}

func TestValidateHost_InvalidStatus(t *testing.T) {
	v := New()

	invalidHost := []byte(`{
		"@context": "https://schema.org",
		"@type": "ComputerSystem",
		"@id": "host-01",
		"name": "test",
		"ipAddress": "192.168.1.10",
		"status": "invalid-status"
	}`)

	result, err := v.ValidateHost(invalidHost)
	require.NoError(t, err)
	assert.False(t, result.Valid)

	hasStatusError := false
	for _, e := range result.Errors {
		if e.Field == "status" {
			hasStatusError = true
			break
		}
	}
	assert.True(t, hasStatusError)
}

func TestValidateContainer_InvalidJSON(t *testing.T) {
	v := New()

	invalidJSON := []byte(`{invalid json}`)

	result, err := v.ValidateContainer(invalidJSON)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
	assert.Equal(t, "document", result.Errors[0].Field)
}

func TestValidateHost_InvalidJSON(t *testing.T) {
	v := New()

	invalidJSON := []byte(`{invalid json}`)

	result, err := v.ValidateHost(invalidJSON)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
	assert.Equal(t, "document", result.Errors[0].Field)
}

func TestIsValidIPAddress(t *testing.T) {
	tests := []struct {
		ip      string
		isValid bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"255.255.255.255", true},
		{"0.0.0.0", true},
		{"999.999.999.999", false},
		{"192.168.1", false},
		{"192.168.1.1.1", false},
		{"not-an-ip", false},
		{"", false},
		{"::1", true}, // IPv6
		{"2001:0db8:85a3:0000:0000:8a2e:0370:7334", true}, // IPv6
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := isValidIPAddress(tt.ip)
			assert.Equal(t, tt.isValid, result, "IP: %s", tt.ip)
		})
	}
}
