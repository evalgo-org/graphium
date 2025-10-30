// Command graphium-dev manages the Graphium development environment stack.
//
// This tool uses EVE's stack orchestration to deploy and manage a
// local development environment with all required services.
//
// Usage:
//
//	# Start development environment
//	go run cmd/graphium-dev/main.go start
//
//	# Stop development environment
//	go run cmd/graphium-dev/main.go stop
//
//	# Remove development environment (including volumes)
//	go run cmd/graphium-dev/main.go remove
//
//	# Check status
//	go run cmd/graphium-dev/main.go status
package main

import (
	"fmt"
	"log"
	"os"

	"eve.evalgo.org/common"
	"eve.evalgo.org/containers/stacks"
	"eve.evalgo.org/containers/stacks/production"
)

const (
	stackFile = "configs/graphium-dev-stack.json"
	stackName = "graphium-dev"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "start":
		startStack()
	case "stop":
		stopStack()
	case "remove":
		removeStack()
	case "status":
		statusStack()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Graphium Development Environment Manager")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  graphium-dev start   - Start development environment")
	fmt.Println("  graphium-dev stop    - Stop development environment")
	fmt.Println("  graphium-dev remove  - Remove development environment (including volumes)")
	fmt.Println("  graphium-dev status  - Check status of development environment")
	fmt.Println()
	fmt.Println("Stack definition: " + stackFile)
}

func startStack() {
	log.Println("🚀 Starting Graphium development environment...")

	// Load stack definition
	stack, err := stacks.LoadStackFromFile(stackFile)
	if err != nil {
		log.Fatalf("Failed to load stack: %v", err)
	}

	// Connect to Docker
	ctx, cli, err := common.CtxCli("unix:///var/run/docker.sock")
	if err != nil {
		log.Fatalf("Failed to connect to Docker: %v", err)
	}
	defer cli.Close()

	// Deploy stack
	deployment, err := production.DeployStack(ctx, cli, stack)
	if err != nil {
		log.Fatalf("Failed to deploy stack: %v", err)
	}

	log.Println("✅ Development environment started successfully!")
	log.Println()
	log.Println("Services:")
	for name, containerID := range deployment.Containers {
		log.Printf("  - %s: %s", name, containerID[:12])
	}
	log.Println()
	log.Println("Connection URLs:")
	log.Println("  CouchDB:")
	log.Println("    - UI:  http://localhost:5984/_utils")
	log.Println("    - API: http://localhost:5984")
	log.Println("    - Credentials: admin / graphium-dev-password")
	log.Println()
	log.Println("To stop: graphium-dev stop")
}

func stopStack() {
	log.Println("🛑 Stopping Graphium development environment...")

	ctx, cli, err := common.CtxCli("unix:///var/run/docker.sock")
	if err != nil {
		log.Fatalf("Failed to connect to Docker: %v", err)
	}
	defer cli.Close()

	if err := production.StopStack(ctx, cli, stackName); err != nil {
		log.Fatalf("Failed to stop stack: %v", err)
	}

	log.Println("✅ Development environment stopped")
}

func removeStack() {
	log.Println("🗑️  Removing Graphium development environment...")

	ctx, cli, err := common.CtxCli("unix:///var/run/docker.sock")
	if err != nil {
		log.Fatalf("Failed to connect to Docker: %v", err)
	}
	defer cli.Close()

	// Remove stack including volumes
	if err := production.RemoveStack(ctx, cli, stackName, true); err != nil {
		log.Fatalf("Failed to remove stack: %v", err)
	}

	log.Println("✅ Development environment removed (including data volumes)")
}

func statusStack() {
	log.Println("📊 Checking Graphium development environment status...")
	log.Println()
	log.Println("To check container status manually:")
	log.Println("  docker ps --filter label=stack=graphium-dev")
	log.Println()
	log.Println("To view logs:")
	log.Println("  docker logs graphium-dev-couchdb")
	log.Println()
	log.Println("To inspect:")
	log.Println("  docker inspect graphium-dev-couchdb")
}
