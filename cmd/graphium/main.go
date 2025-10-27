package main

import (
	"fmt"
	"os"

	"evalgo.org/graphium/internal/commands"
	"evalgo.org/graphium/internal/version"
)

// @title Graphium API
// @version 0.1.0
// @description Graphium is a semantic container orchestration platform that uses knowledge graphs and JSON-LD to manage multi-host Docker infrastructure with intelligent querying, graph traversal, and real-time insights.
// @description
// @description ## Features
// @description - JSON-LD/Schema.org semantic models
// @description - REST API for container and host management
// @description - Graph visualization and traversal
// @description - Real-time Docker agent integration
// @description - WebSocket support for live updates
// @description
// @description ## Authentication
// @description All API endpoints require JWT token authentication. Use the /api/v1/auth/login endpoint to obtain a token.
// @description Include the token in the Authorization header: `Authorization: Bearer <token>`

// @contact.name Graphium API Support
// @contact.url https://github.com/[org]/graphium
// @contact.email support@graphium.io

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8095
// @BasePath /api/v1

// @schemes http https

// @tag.name Containers
// @tag.description Operations related to container management

// @tag.name Hosts
// @tag.description Operations related to host management

// @tag.name Queries
// @tag.description Graph query and traversal operations

// @tag.name Statistics
// @tag.description Statistics and metrics endpoints

// @tag.name Graph
// @tag.description Graph visualization endpoints

// @tag.name WebSocket
// @tag.description WebSocket endpoints for real-time updates

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
