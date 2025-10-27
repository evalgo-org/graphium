package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"evalgo.org/graphium/internal/validation"
	"github.com/spf13/cobra"
)

var (
	validateLocal bool
)

var validateCmd = &cobra.Command{
	Use:   "validate [type] [file]",
	Short: "Validate a JSON-LD document",
	Long: `Validate a JSON-LD document against Graphium schemas.

Examples:
  graphium validate container my-container.json
  graphium validate host my-host.json --local
  graphium validate container web-app.json`,
	Args:  cobra.ExactArgs(2),
	RunE:  runValidate,
}

func init() {
	validateCmd.Flags().BoolVar(&validateLocal, "local", true, "validate locally (default: true)")
}

func runValidate(cmd *cobra.Command, args []string) error {
	entityType := args[0]
	filename := args[1]

	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Use local validation by default
	if validateLocal {
		return runLocalValidation(entityType, data)
	}

	// Use API validation
	return runAPIValidation(entityType, data)
}

// runLocalValidation validates the document locally
func runLocalValidation(entityType string, data []byte) error {
	validator := validation.New()

	var result *validation.ValidationResult
	var err error

	switch entityType {
	case "container":
		result, err = validator.ValidateContainer(data)
	case "host":
		result, err = validator.ValidateHost(data)
	default:
		return fmt.Errorf("unknown entity type: %s (use 'container' or 'host')", entityType)
	}

	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	// Print results
	if result.Valid {
		fmt.Println("✓ Document is valid")
		return nil
	}

	fmt.Println("✗ Validation failed:")
	for _, e := range result.Errors {
		if e.Value != nil {
			fmt.Printf("  - %s: %s (value: %v)\n", e.Field, e.Message, e.Value)
		} else {
			fmt.Printf("  - %s: %s\n", e.Field, e.Message)
		}
	}

	return fmt.Errorf("validation failed")
}

// runAPIValidation validates the document via API
func runAPIValidation(entityType string, data []byte) error {
	apiURL := cfg.Agent.APIURL
	if apiURL == "" {
		apiURL = fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port)
	}

	url := fmt.Sprintf("%s/api/v1/validate/%s", apiURL, entityType)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to connect to API: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result validation.ValidationResult
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Valid {
		fmt.Println("✓ Document is valid")
		return nil
	}

	fmt.Println("✗ Validation failed:")
	for _, e := range result.Errors {
		if e.Value != nil {
			fmt.Printf("  - %s: %s (value: %v)\n", e.Field, e.Message, e.Value)
		} else {
			fmt.Printf("  - %s: %s\n", e.Field, e.Message)
		}
	}

	return fmt.Errorf("validation failed")
}
