package main

import (
	"fmt"
	"os"

	"evalgo.org/graphium/internal/commands"
	"evalgo.org/graphium/internal/version"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	version.Version = Version
	version.BuildTime = BuildTime
	version.GitCommit = GitCommit

	if err := commands.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
