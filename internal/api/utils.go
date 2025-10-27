package api

import (
	"fmt"
	"strings"
	"time"
)

// generateID generates a unique ID for a resource.
func generateID(resourceType, name string) string {
	// Sanitize name
	sanitized := strings.ToLower(name)
	sanitized = strings.ReplaceAll(sanitized, " ", "-")
	sanitized = strings.ReplaceAll(sanitized, "_", "-")

	// Generate timestamp suffix for uniqueness
	timestamp := time.Now().Unix()

	return fmt.Sprintf("%s-%s-%d", resourceType, sanitized, timestamp)
}
