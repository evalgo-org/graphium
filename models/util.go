package models

import (
	"fmt"

	"github.com/google/uuid"
)

// GenerateID generates a unique ID with the given prefix
// Example: GenerateID("action") -> "action:uuid-here"
func GenerateID(prefix string) string {
	return fmt.Sprintf("%s:%s", prefix, uuid.New().String())
}
