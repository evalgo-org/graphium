package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"eve.evalgo.org/db"
	"github.com/spf13/cobra"

	"evalgo.org/graphium/internal/integrity"
)

var integrityCmd = &cobra.Command{
	Use:   "integrity",
	Short: "Database integrity checking and repair",
	Long:  `Scan, validate, and repair CouchDB integrity issues`,
}

var integrityHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check database health",
	Long:  `Perform a quick health check and display the health score`,
	RunE:  runIntegrityHealth,
}

var integrityScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan for integrity issues",
	Long:  `Perform a comprehensive scan for duplicates, conflicts, and validation errors`,
	RunE:  runIntegrityScan,
}

var integrityPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Create a repair plan",
	Long:  `Generate a repair plan to fix detected integrity issues`,
	RunE:  runIntegrityPlan,
}

var integrityRepairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Execute repair operations",
	Long:  `Execute a repair plan to fix integrity issues`,
	RunE:  runIntegrityRepair,
}

func init() {
	// Add flags for scan command
	integrityScanCmd.Flags().Bool("duplicates", true, "Scan for duplicate documents")
	integrityScanCmd.Flags().Bool("conflicts", true, "Scan for revision conflicts")
	integrityScanCmd.Flags().Bool("references", true, "Scan for reference integrity")
	integrityScanCmd.Flags().Bool("schemas", false, "Scan for schema validation errors")
	integrityScanCmd.Flags().StringSlice("types", []string{}, "Document types to scan (empty = all)")
	integrityScanCmd.Flags().Bool("json", false, "Output results as JSON")

	// Add flags for plan command
	integrityPlanCmd.Flags().String("strategy", "latest_wins", "Resolution strategy (latest_wins, highest_rev, merge, manual)")
	integrityPlanCmd.Flags().StringSlice("risk", []string{"low", "medium", "high"}, "Risk levels to include (low, medium, high)")
	integrityPlanCmd.Flags().Bool("json", false, "Output plan as JSON")

	// Add flags for repair command
	integrityRepairCmd.Flags().Bool("dry-run", true, "Perform a dry-run without making actual changes")
	integrityRepairCmd.Flags().Bool("yes", false, "Skip confirmation prompt")
	integrityRepairCmd.Flags().String("strategy", "latest_wins", "Resolution strategy (latest_wins, highest_rev, merge, manual)")
	integrityRepairCmd.Flags().StringSlice("risk", []string{"low", "medium"}, "Risk levels to include (low, medium, high)")

	// Add subcommands
	integrityCmd.AddCommand(integrityHealthCmd)
	integrityCmd.AddCommand(integrityScanCmd)
	integrityCmd.AddCommand(integrityPlanCmd)
	integrityCmd.AddCommand(integrityRepairCmd)
}

func runIntegrityHealth(cmd *cobra.Command, args []string) error {
	fmt.Println("🏥 Checking Database Health")
	fmt.Println()

	// Initialize database connection
	dbService, err := initDatabase()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create integrity service
	logger := log.New(os.Stdout, "[integrity] ", log.LstdFlags)
	integrityService, err := integrity.NewService(dbService, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create integrity service: %w", err)
	}
	defer integrityService.Close()

	// Perform health check
	ctx := context.Background()
	health, err := integrityService.CheckHealth(ctx)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	// Display results
	fmt.Printf("Timestamp:       %s\n", health.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Total Documents: %d\n", health.TotalDocuments)
	fmt.Printf("Issues Found:    %d\n", health.IssueCount)
	fmt.Println()

	// Health score with color
	scoreColor := getScoreColor(health.HealthScore)
	fmt.Printf("Health Score:    %s%d/100%s\n", scoreColor, health.HealthScore, colorReset)
	fmt.Println()

	// Issues by type
	if len(health.IssuesByType) > 0 {
		fmt.Println("Issues by Type:")
		for issueType, count := range health.IssuesByType {
			fmt.Printf("  %s: %d\n", issueType, count)
		}
		fmt.Println()
	}

	// Issues by severity
	if len(health.IssuesBySeverity) > 0 {
		fmt.Println("Issues by Severity:")
		for severity, count := range health.IssuesBySeverity {
			fmt.Printf("  %s: %d\n", severity, count)
		}
		fmt.Println()
	}

	// Recommendations
	if len(health.Recommendations) > 0 {
		fmt.Println("Recommendations:")
		for _, rec := range health.Recommendations {
			fmt.Printf("  • %s\n", rec)
		}
		fmt.Println()
	}

	// Exit with non-zero if health is poor
	if health.HealthScore < 50 {
		return fmt.Errorf("database health is critical (score: %d)", health.HealthScore)
	}

	return nil
}

func runIntegrityScan(cmd *cobra.Command, args []string) error {
	fmt.Println("🔍 Scanning for Integrity Issues")
	fmt.Println()

	// Get flags
	scanDuplicates, _ := cmd.Flags().GetBool("duplicates")
	scanConflicts, _ := cmd.Flags().GetBool("conflicts")
	scanReferences, _ := cmd.Flags().GetBool("references")
	scanSchemas, _ := cmd.Flags().GetBool("schemas")
	documentTypes, _ := cmd.Flags().GetStringSlice("types")
	outputJSON, _ := cmd.Flags().GetBool("json")

	// Initialize database connection
	dbService, err := initDatabase()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create integrity service
	logger := log.New(os.Stdout, "[integrity] ", log.LstdFlags)
	integrityService, err := integrity.NewService(dbService, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create integrity service: %w", err)
	}
	defer integrityService.Close()

	// Prepare scan options
	options := integrity.ScanOptions{
		ScanDuplicates: scanDuplicates,
		ScanConflicts:  scanConflicts,
		ScanReferences: scanReferences,
		ScanSchemas:    scanSchemas,
		DocumentTypes:  documentTypes,
	}

	// Perform scan
	ctx := context.Background()
	report, err := integrityService.Scan(ctx, options)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Output results
	if outputJSON {
		// JSON output
		jsonData, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonData))
	} else {
		// Human-readable output
		fmt.Printf("Scan ID:          %s\n", report.ID)
		fmt.Printf("Duration:         %v\n", report.Duration)
		fmt.Printf("Documents Scanned: %d\n", report.DocumentsScanned)
		fmt.Printf("Issues Found:     %d\n", report.Summary.TotalIssues)
		fmt.Println()

		// Health score with color
		scoreColor := getScoreColor(report.Summary.HealthScore)
		fmt.Printf("Health Score:     %s%d/100%s\n", scoreColor, report.Summary.HealthScore, colorReset)
		fmt.Println()

		// Issues by type
		if len(report.Summary.ByType) > 0 {
			fmt.Println("Issues by Type:")
			for issueType, count := range report.Summary.ByType {
				fmt.Printf("  %s: %d\n", issueType, count)
			}
			fmt.Println()
		}

		// Issues by severity
		if len(report.Summary.BySeverity) > 0 {
			fmt.Println("Issues by Severity:")
			for severity, count := range report.Summary.BySeverity {
				severityColor := getSeverityColor(severity)
				fmt.Printf("  %s%s%s: %d\n", severityColor, severity, colorReset, count)
			}
			fmt.Println()
		}

		// Detailed issues
		if len(report.IssuesFound) > 0 {
			fmt.Println("Detailed Issues:")
			for i, issue := range report.IssuesFound {
				if i >= 10 {
					fmt.Printf("  ... and %d more issues\n", len(report.IssuesFound)-10)
					break
				}
				severityColor := getSeverityColor(issue.Severity)
				fmt.Printf("\n  Issue #%d:\n", i+1)
				fmt.Printf("    Type:        %s\n", issue.Type)
				fmt.Printf("    Severity:    %s%s%s\n", severityColor, issue.Severity, colorReset)
				fmt.Printf("    Document ID: %s\n", issue.DocumentID)
				fmt.Printf("    Description: %s\n", issue.Description)
			}
			fmt.Println()
		}

		// Next steps
		if report.Summary.TotalIssues > 0 {
			fmt.Println("Next Steps:")
			fmt.Println("  1. Review the issues above")
			fmt.Println("  2. Run 'graphium integrity plan' to create a repair plan")
			fmt.Println("  3. Execute the plan with 'graphium integrity repair'")
			fmt.Println()
		} else {
			fmt.Println("✅ No integrity issues found!")
			fmt.Println()
		}
	}

	// Exit with non-zero if issues found
	if report.Summary.TotalIssues > 0 {
		return fmt.Errorf("found %d integrity issues", report.Summary.TotalIssues)
	}

	return nil
}

// initDatabase creates a database connection
func initDatabase() (*db.CouchDBService, error) {
	couchConfig := db.CouchDBConfig{
		URL:             cfg.CouchDB.URL,
		Database:        cfg.CouchDB.Database,
		Username:        cfg.CouchDB.Username,
		Password:        cfg.CouchDB.Password,
		CreateIfMissing: false,
	}

	return db.NewCouchDBServiceFromConfig(couchConfig)
}

// Color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorGreen  = "\033[32m"
	colorOrange = "\033[38;5;208m"
)

// getScoreColor returns the appropriate color for a health score
func getScoreColor(score int) string {
	if score >= 90 {
		return colorGreen
	} else if score >= 70 {
		return colorYellow
	} else if score >= 50 {
		return colorOrange
	}
	return colorRed
}

// getSeverityColor returns the appropriate color for a severity level
func getSeverityColor(severity integrity.Severity) string {
	switch severity {
	case integrity.SeverityCritical:
		return colorRed
	case integrity.SeverityHigh:
		return colorOrange
	case integrity.SeverityMedium:
		return colorYellow
	case integrity.SeverityLow:
		return colorGreen
	default:
		return colorReset
	}
}

func runIntegrityPlan(cmd *cobra.Command, args []string) error {
	fmt.Println("🔧 Creating Repair Plan")
	fmt.Println()

	// Get flags
	strategyStr, _ := cmd.Flags().GetString("strategy")
	riskLevels, _ := cmd.Flags().GetStringSlice("risk")
	outputJSON, _ := cmd.Flags().GetBool("json")

	// Parse strategy
	strategy := integrity.ResolutionStrategy(strategyStr)

	// Parse risk levels
	var riskFilter []integrity.RiskLevel
	for _, risk := range riskLevels {
		riskFilter = append(riskFilter, integrity.RiskLevel(risk))
	}

	// Initialize database connection
	dbService, err := initDatabase()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create integrity service
	logger := log.New(os.Stdout, "[integrity] ", log.LstdFlags)
	integrityService, err := integrity.NewService(dbService, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create integrity service: %w", err)
	}
	defer integrityService.Close()

	// Create repair plan
	plan, err := integrityService.CreateRepairPlan("latest", strategy, riskFilter)
	if err != nil {
		return fmt.Errorf("failed to create repair plan: %w", err)
	}

	// Output results
	if outputJSON {
		// JSON output
		jsonData, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonData))
	} else {
		// Human-readable output
		fmt.Printf("Plan ID:           %s\n", plan.ID)
		fmt.Printf("Strategy:          %s\n", plan.Strategy)
		fmt.Printf("Operations:        %d\n", len(plan.Operations))
		fmt.Printf("Estimated Duration: %dms\n", plan.EstimatedDuration)
		fmt.Printf("Dry-Run:           %v\n", plan.DryRun)
		fmt.Println()

		// Show operations summary
		if len(plan.Operations) > 0 {
			fmt.Println("Operations by Type:")
			opTypes := make(map[integrity.OperationType]int)
			for _, op := range plan.Operations {
				opTypes[op.Type]++
			}
			for opType, count := range opTypes {
				fmt.Printf("  %s: %d\n", opType, count)
			}
			fmt.Println()

			// Show first few operations
			fmt.Println("Sample Operations:")
			for i, op := range plan.Operations {
				if i >= 5 {
					fmt.Printf("  ... and %d more operations\n", len(plan.Operations)-5)
					break
				}
				riskColor := getRiskColor(op.Risk)
				fmt.Printf("  %d. [%s%s%s] %s\n", i+1, riskColor, op.Risk, colorReset, op.Action)
			}
			fmt.Println()
		}

		// Next steps
		if len(plan.Operations) > 0 {
			fmt.Println("Next Steps:")
			fmt.Println("  1. Review the operations above")
			fmt.Println("  2. Run 'graphium integrity repair --dry-run=true' to test")
			fmt.Println("  3. Run 'graphium integrity repair --dry-run=false' to execute")
			fmt.Println()
		} else {
			fmt.Println("✅ No repair operations needed!")
			fmt.Println()
		}
	}

	return nil
}

func runIntegrityRepair(cmd *cobra.Command, args []string) error {
	// Get flags
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	skipConfirm, _ := cmd.Flags().GetBool("yes")
	strategyStr, _ := cmd.Flags().GetString("strategy")
	riskLevels, _ := cmd.Flags().GetStringSlice("risk")

	if dryRun {
		fmt.Println("🔍 Dry-Run Mode: Simulating Repairs")
	} else {
		fmt.Println("⚠️  Live Mode: Executing Repairs")
	}
	fmt.Println()

	// Parse strategy
	strategy := integrity.ResolutionStrategy(strategyStr)

	// Parse risk levels
	var riskFilter []integrity.RiskLevel
	for _, risk := range riskLevels {
		riskFilter = append(riskFilter, integrity.RiskLevel(risk))
	}

	// Initialize database connection
	dbService, err := initDatabase()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create integrity service
	logger := log.New(os.Stdout, "[integrity] ", log.LstdFlags)
	integrityService, err := integrity.NewService(dbService, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create integrity service: %w", err)
	}
	defer integrityService.Close()

	// Create repair plan
	fmt.Println("Creating repair plan...")
	plan, err := integrityService.CreateRepairPlan("latest", strategy, riskFilter)
	if err != nil {
		return fmt.Errorf("failed to create repair plan: %w", err)
	}
	plan.DryRun = dryRun

	fmt.Printf("Found %d operations to execute\n\n", len(plan.Operations))

	if len(plan.Operations) == 0 {
		fmt.Println("✅ No repairs needed!")
		return nil
	}

	// Confirm before executing (unless --yes flag)
	if !dryRun && !skipConfirm {
		fmt.Printf("⚠️  WARNING: This will modify %d documents in the database!\n", len(plan.Operations))
		fmt.Print("Are you sure you want to continue? (yes/no): ")
		var response string
		fmt.Scanln(&response)
		if response != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
		fmt.Println()
	}

	// Execute plan
	ctx := context.Background()
	result, err := integrityService.ExecutePlan(ctx, plan)
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	// Display results
	fmt.Println()
	fmt.Printf("Execution ID:      %s\n", result.ExecutionID)
	fmt.Printf("Duration:          %v\n", result.Duration)
	fmt.Printf("Operations:        %d total\n", len(result.Operations))
	fmt.Printf("Successful:        %s%d%s\n", colorGreen, result.SuccessCount, colorReset)
	fmt.Printf("Failed:            %s%d%s\n", colorRed, result.FailureCount, colorReset)
	fmt.Printf("Dry-Run:           %v\n", result.DryRun)

	if result.Aborted {
		fmt.Printf("Status:            %sABORTED%s\n", colorRed, colorReset)
		fmt.Printf("Reason:            %v\n", result.AbortReason)
	}
	fmt.Println()

	// Show failures if any
	if result.FailureCount > 0 {
		fmt.Println("Failed Operations:")
		for i, opResult := range result.Operations {
			if !opResult.Success {
				fmt.Printf("  %d. %s - %v\n", i+1, opResult.Operation.DocumentID, opResult.Error)
			}
		}
		fmt.Println()
	}

	// Summary
	if result.DryRun {
		fmt.Println("✅ Dry-run completed successfully!")
		fmt.Println("Run with --dry-run=false to execute actual repairs.")
	} else if result.FailureCount > 0 {
		return fmt.Errorf("%d operations failed", result.FailureCount)
	} else {
		fmt.Println("✅ All repairs completed successfully!")
	}

	return nil
}

// getRiskColor returns the appropriate color for a risk level
func getRiskColor(risk integrity.RiskLevel) string {
	switch risk {
	case integrity.RiskHigh:
		return colorRed
	case integrity.RiskMedium:
		return colorYellow
	case integrity.RiskLow:
		return colorGreen
	default:
		return colorReset
	}
}
