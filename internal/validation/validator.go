package validation

import (
	"encoding/json"
	"fmt"
	"strings"

	"evalgo.org/graphium/models"
	"github.com/go-playground/validator/v10"
	"github.com/piprate/json-gold/ld"
)

// Validator handles JSON-LD document validation
type Validator struct {
	structValidator *validator.Validate
	jsonldProcessor *ld.JsonLdProcessor
}

// ValidationError represents a single validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

// ValidationResult represents the result of validation
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// New creates a new validator instance
func New() *Validator {
	return &Validator{
		structValidator: validator.New(),
		jsonldProcessor: ld.NewJsonLdProcessor(),
	}
}

// ValidateContainer validates a container JSON-LD document
func (v *Validator) ValidateContainer(data []byte) (*ValidationResult, error) {
	var container models.Container

	// Parse JSON
	if err := json.Unmarshal(data, &container); err != nil {
		return &ValidationResult{
			Valid: false,
			Errors: []ValidationError{
				{
					Field:   "document",
					Message: fmt.Sprintf("Invalid JSON: %v", err),
				},
			},
		}, nil
	}

	// Validate JSON-LD structure
	jsonldErrors := v.validateJSONLD(data)

	// Validate container-specific fields
	containerErrors := v.validateContainerFields(&container)

	// Combine errors
	allErrors := append(jsonldErrors, containerErrors...)

	return &ValidationResult{
		Valid:  len(allErrors) == 0,
		Errors: allErrors,
	}, nil
}

// ValidateHost validates a host JSON-LD document
func (v *Validator) ValidateHost(data []byte) (*ValidationResult, error) {
	var host models.Host

	// Parse JSON
	if err := json.Unmarshal(data, &host); err != nil {
		return &ValidationResult{
			Valid: false,
			Errors: []ValidationError{
				{
					Field:   "document",
					Message: fmt.Sprintf("Invalid JSON: %v", err),
				},
			},
		}, nil
	}

	// Validate JSON-LD structure
	jsonldErrors := v.validateJSONLD(data)

	// Validate host-specific fields
	hostErrors := v.validateHostFields(&host)

	// Combine errors
	allErrors := append(jsonldErrors, hostErrors...)

	return &ValidationResult{
		Valid:  len(allErrors) == 0,
		Errors: allErrors,
	}, nil
}

// validateJSONLD validates JSON-LD structure using json-gold
func (v *Validator) validateJSONLD(data []byte) []ValidationError {
	var errors []ValidationError

	// Parse as generic JSON
	var doc interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		errors = append(errors, ValidationError{
			Field:   "document",
			Message: fmt.Sprintf("Invalid JSON: %v", err),
		})
		return errors
	}

	// Check @context
	if docMap, ok := doc.(map[string]interface{}); ok {
		// Validate @context exists
		if _, hasContext := docMap["@context"]; !hasContext {
			errors = append(errors, ValidationError{
				Field:   "@context",
				Message: "Missing @context field (required for JSON-LD)",
			})
		}

		// Validate @type exists
		if _, hasType := docMap["@type"]; !hasType {
			errors = append(errors, ValidationError{
				Field:   "@type",
				Message: "Missing @type field (required for JSON-LD)",
			})
		}

		// Validate @id exists
		if _, hasID := docMap["@id"]; !hasID {
			errors = append(errors, ValidationError{
				Field:   "@id",
				Message: "Missing @id field (required for JSON-LD)",
			})
		}

		// Try to expand the JSON-LD to validate it's well-formed
		options := ld.NewJsonLdOptions("")
		_, err := v.jsonldProcessor.Expand(doc, options)
		if err != nil {
			errors = append(errors, ValidationError{
				Field:   "document",
				Message: fmt.Sprintf("Invalid JSON-LD structure: %v", err),
			})
		}
	}

	return errors
}

// validateContainerFields validates container-specific business logic
func (v *Validator) validateContainerFields(container *models.Container) []ValidationError {
	var errors []ValidationError

	// Required fields
	if container.Name == "" {
		errors = append(errors, ValidationError{
			Field:   "name",
			Message: "Name is required",
		})
	}

	if container.Image == "" {
		errors = append(errors, ValidationError{
			Field:   "executableName",
			Message: "Image (executableName) is required",
		})
	}

	if container.HostedOn == "" {
		errors = append(errors, ValidationError{
			Field:   "hostedOn",
			Message: "HostedOn is required (must reference a host)",
		})
	}

	// Validate @type
	if container.Type != "" && container.Type != "SoftwareApplication" &&
	   container.Type != "Container" {
		errors = append(errors, ValidationError{
			Field:   "@type",
			Message: "Type must be 'SoftwareApplication' or 'Container'",
			Value:   container.Type,
		})
	}

	// Validate status
	validStatuses := map[string]bool{
		"running":   true,
		"stopped":   true,
		"paused":    true,
		"restarting": true,
		"exited":    true,
		"created":   true,
	}

	if container.Status != "" && !validStatuses[container.Status] {
		errors = append(errors, ValidationError{
			Field:   "status",
			Message: fmt.Sprintf("Invalid status: must be one of: %s",
				strings.Join([]string{"running", "stopped", "paused", "restarting", "exited", "created"}, ", ")),
			Value:   container.Status,
		})
	}

	// Validate ports
	for i, port := range container.Ports {
		if port.HostPort < 0 || port.HostPort > 65535 {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("ports[%d].hostPort", i),
				Message: "Port must be between 0 and 65535",
				Value:   port.HostPort,
			})
		}

		if port.ContainerPort < 0 || port.ContainerPort > 65535 {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("ports[%d].containerPort", i),
				Message: "Port must be between 0 and 65535",
				Value:   port.ContainerPort,
			})
		}

		validProtocols := map[string]bool{"tcp": true, "udp": true, "sctp": true}
		if port.Protocol != "" && !validProtocols[strings.ToLower(port.Protocol)] {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("ports[%d].protocol", i),
				Message: "Protocol must be 'tcp', 'udp', or 'sctp'",
				Value:   port.Protocol,
			})
		}
	}

	return errors
}

// validateHostFields validates host-specific business logic
func (v *Validator) validateHostFields(host *models.Host) []ValidationError {
	var errors []ValidationError

	// Required fields
	if host.Name == "" {
		errors = append(errors, ValidationError{
			Field:   "name",
			Message: "Name is required",
		})
	}

	if host.IPAddress == "" {
		errors = append(errors, ValidationError{
			Field:   "ipAddress",
			Message: "IP address is required",
		})
	}

	// Validate @type
	if host.Type != "" && host.Type != "ComputerSystem" &&
	   host.Type != "Server" && host.Type != "Host" {
		errors = append(errors, ValidationError{
			Field:   "@type",
			Message: "Type must be 'ComputerSystem', 'Server', or 'Host'",
			Value:   host.Type,
		})
	}

	// Validate status
	validStatuses := map[string]bool{
		"active":       true,
		"inactive":     true,
		"maintenance":  true,
		"unreachable":  true,
	}

	if host.Status != "" && !validStatuses[host.Status] {
		errors = append(errors, ValidationError{
			Field:   "status",
			Message: fmt.Sprintf("Invalid status: must be one of: %s",
				strings.Join([]string{"active", "inactive", "maintenance", "unreachable"}, ", ")),
			Value:   host.Status,
		})
	}

	// Validate IP address format (basic check)
	if host.IPAddress != "" && !isValidIPAddress(host.IPAddress) {
		errors = append(errors, ValidationError{
			Field:   "ipAddress",
			Message: "Invalid IP address format",
			Value:   host.IPAddress,
		})
	}

	// Validate CPU count
	if host.CPU < 0 {
		errors = append(errors, ValidationError{
			Field:   "cpu",
			Message: "CPU count cannot be negative",
			Value:   host.CPU,
		})
	}

	// Validate memory
	if host.Memory < 0 {
		errors = append(errors, ValidationError{
			Field:   "memory",
			Message: "Memory size cannot be negative",
			Value:   host.Memory,
		})
	}

	return errors
}

// isValidIPAddress performs basic IP address validation
func isValidIPAddress(ip string) bool {
	// Basic check for IPv4 format
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		// Could be IPv6, which is more complex - accept for now
		if strings.Contains(ip, ":") {
			return true
		}
		return false
	}

	for _, part := range parts {
		if len(part) == 0 || len(part) > 3 {
			return false
		}

		var num int
		if _, err := fmt.Sscanf(part, "%d", &num); err != nil {
			return false
		}

		if num < 0 || num > 255 {
			return false
		}
	}

	return true
}
