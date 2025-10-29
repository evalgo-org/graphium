package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"evalgo.org/graphium/internal/config"
	"evalgo.org/graphium/internal/version"
)

var (
	cfgFile string
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "graphium",
	Short: "The Essential Element for Container Intelligence",
	Long: `Graphium is a semantic container orchestration platform that uses
knowledge graphs to manage multi-host Docker infrastructure.

Query your containers with JSON-LD, explore dependencies with graph
traversal, and gain real-time insights through an intuitive web UI.`,
	Version: version.Version,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./config.yaml)")
	rootCmd.PersistentFlags().String("log-level", "", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().String("log-format", "", "log format (json, text)")

	// These should never fail as flags are defined above
	_ = viper.BindPFlag("logging.level", rootCmd.PersistentFlags().Lookup("log-level"))   //nolint:errcheck
	_ = viper.BindPFlag("logging.format", rootCmd.PersistentFlags().Lookup("log-format")) //nolint:errcheck

	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(stackCmd)
	rootCmd.AddCommand(versionCmd)

	rootCmd.SetVersionTemplate(`{{with .Name}}{{printf "%s " .}}{{end}}{{printf "%s" .Version}}
`)
}

func initConfig() {
	var err error
	cfg, err = config.Load(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		info := version.Get()
		fmt.Println(info.String())

		if cmd.Flag("verbose").Changed {
			fmt.Printf("\nDetails:\n")
			fmt.Printf("  Version:    %s\n", info.Version)
			fmt.Printf("  Git Commit: %s\n", info.GitCommit)
			fmt.Printf("  Built:      %s\n", info.BuildTime)
			fmt.Printf("  Go Version: %s\n", info.GoVersion)
			fmt.Printf("  Platform:   %s\n", info.Platform)
		}
	},
}

func init() {
	versionCmd.Flags().BoolP("verbose", "v", false, "verbose version output")
}
