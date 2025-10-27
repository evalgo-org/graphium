package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"evalgo.org/graphium/internal/storage"
	"eve.evalgo.org/db"
	"github.com/spf13/cobra"
)

var (
	// Query flags
	queryLimit      int
	queryStatus     string
	queryHost       string
	queryDatacenter string
	queryFormat     string
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query the container graph",
	Long:  `Execute semantic queries against the container graph`,
}

var listCmd = &cobra.Command{
	Use:   "list [type]",
	Short: "List entities of a type (containers, hosts)",
	Long: `List entities with optional filtering.

Examples:
  graphium query list containers
  graphium query list containers --status running
  graphium query list containers --host host-01
  graphium query list hosts --datacenter us-east
  graphium query list hosts --status active --format json`,
	Args: cobra.ExactArgs(1),
	RunE: runList,
}

var traverseCmd = &cobra.Command{
	Use:   "traverse [id]",
	Short: "Traverse dependency graph from a container",
	Long: `Follow dependency relationships to visualize the dependency tree.

Examples:
  graphium query traverse nginx-web
  graphium query traverse postgres-db --depth 3
  graphium query traverse api-gateway --format json`,
	Args: cobra.ExactArgs(1),
	RunE: runTraverse,
}

var dependentsCmd = &cobra.Command{
	Use:   "dependents [id]",
	Short: "Find what depends on a container (reverse lookup)",
	Long: `Find all containers that depend on the specified container.

Examples:
  graphium query dependents postgres-db
  graphium query dependents redis-cache --format json`,
	Args: cobra.ExactArgs(1),
	RunE: runDependents,
}

var topologyCmd = &cobra.Command{
	Use:   "topology [datacenter]",
	Short: "Show datacenter topology",
	Long: `Display the complete infrastructure topology for a datacenter.

Examples:
  graphium query topology us-east
  graphium query topology eu-west --format json`,
	Args: cobra.ExactArgs(1),
	RunE: runTopology,
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show infrastructure statistics",
	Long:  `Display aggregated statistics about the infrastructure.`,
	RunE:  runStats,
}

func init() {
	queryCmd.AddCommand(listCmd)
	queryCmd.AddCommand(traverseCmd)
	queryCmd.AddCommand(dependentsCmd)
	queryCmd.AddCommand(topologyCmd)
	queryCmd.AddCommand(statsCmd)

	// List command flags
	listCmd.Flags().IntVar(&queryLimit, "limit", 100, "maximum results")
	listCmd.Flags().StringVar(&queryStatus, "status", "", "filter by status")
	listCmd.Flags().StringVar(&queryHost, "host", "", "filter by host")
	listCmd.Flags().StringVar(&queryDatacenter, "datacenter", "", "filter by datacenter")
	listCmd.Flags().StringVar(&queryFormat, "format", "table", "output format (table, json)")

	// Traverse command flags
	traverseCmd.Flags().IntVar(&queryLimit, "depth", 5, "maximum traversal depth")
	traverseCmd.Flags().StringVar(&queryFormat, "format", "tree", "output format (tree, json)")

	// Dependents command flags
	dependentsCmd.Flags().StringVar(&queryFormat, "format", "table", "output format (table, json)")

	// Topology command flags
	topologyCmd.Flags().StringVar(&queryFormat, "format", "tree", "output format (tree, json)")

	// Stats command flags
	statsCmd.Flags().StringVar(&queryFormat, "format", "table", "output format (table, json)")
}

func runList(cmd *cobra.Command, args []string) error {
	entityType := args[0]

	// Initialize storage
	store, err := storage.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// Build filters
	filters := make(map[string]interface{})
	if queryStatus != "" {
		filters["status"] = queryStatus
	}
	if queryHost != "" {
		filters["hostedOn"] = queryHost
	}
	if queryDatacenter != "" {
		filters["location"] = queryDatacenter
	}

	switch strings.ToLower(entityType) {
	case "containers", "container":
		containers, err := store.ListContainers(filters)
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}

		if queryFormat == "json" {
			return printJSON(containers)
		}

		// Print table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tIMAGE\tSTATUS\tHOST")
		for _, c := range containers {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				c.ID, c.Name, c.Image, c.Status, c.HostedOn)
		}
		w.Flush()
		fmt.Printf("\nTotal: %d containers\n", len(containers))

	case "hosts", "host":
		hosts, err := store.ListHosts(filters)
		if err != nil {
			return fmt.Errorf("failed to list hosts: %w", err)
		}

		if queryFormat == "json" {
			return printJSON(hosts)
		}

		// Print table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tIP ADDRESS\tSTATUS\tDATACENTER")
		for _, h := range hosts {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				h.ID, h.Name, h.IPAddress, h.Status, h.Datacenter)
		}
		w.Flush()
		fmt.Printf("\nTotal: %d hosts\n", len(hosts))

	default:
		return fmt.Errorf("unknown entity type: %s (use 'containers' or 'hosts')", entityType)
	}

	return nil
}

func runTraverse(cmd *cobra.Command, args []string) error {
	id := args[0]

	// Initialize storage
	store, err := storage.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// Get dependency graph
	graph, err := store.GetContainerDependencyGraph(id, queryLimit)
	if err != nil {
		return fmt.Errorf("failed to traverse graph: %w", err)
	}

	if queryFormat == "json" {
		return printJSON(graph)
	}

	// Print tree
	fmt.Printf("Dependency graph for: %s\n\n", id)
	printGraph(graph, 0)

	return nil
}

func runDependents(cmd *cobra.Command, args []string) error {
	id := args[0]

	// Initialize storage
	store, err := storage.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// Get dependents
	dependents, err := store.GetContainerDependents(id)
	if err != nil {
		return fmt.Errorf("failed to get dependents: %w", err)
	}

	if queryFormat == "json" {
		return printJSON(dependents)
	}

	// Print table
	if len(dependents) == 0 {
		fmt.Printf("No containers depend on: %s\n", id)
		return nil
	}

	fmt.Printf("Containers that depend on: %s\n\n", id)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tHOST")
	for _, c := range dependents {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			c.ID, c.Name, c.Status, c.HostedOn)
	}
	w.Flush()
	fmt.Printf("\nTotal: %d dependents\n", len(dependents))

	return nil
}

func runTopology(cmd *cobra.Command, args []string) error {
	datacenter := args[0]

	// Initialize storage
	store, err := storage.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// Get topology
	topology, err := store.GetDatacenterTopology(datacenter)
	if err != nil {
		return fmt.Errorf("failed to get topology: %w", err)
	}

	if queryFormat == "json" {
		return printJSON(topology)
	}

	// Print tree
	fmt.Printf("Datacenter: %s\n\n", topology.Datacenter)

	if len(topology.Hosts) == 0 {
		fmt.Println("No hosts found in this datacenter")
		return nil
	}

	totalContainers := 0
	for hostID, hostTopo := range topology.Hosts {
		containerCount := len(hostTopo.Containers)
		totalContainers += containerCount

		fmt.Printf("└─ %s (%s)\n", hostTopo.Host.Name, hostID)
		fmt.Printf("   ├─ IP: %s\n", hostTopo.Host.IPAddress)
		fmt.Printf("   ├─ Status: %s\n", hostTopo.Host.Status)
		fmt.Printf("   └─ Containers: %d\n", containerCount)

		for i, container := range hostTopo.Containers {
			prefix := "      "
			if i == containerCount-1 {
				fmt.Printf("%s└─ %s (%s)\n", prefix, container.Name, container.Status)
			} else {
				fmt.Printf("%s├─ %s (%s)\n", prefix, container.Name, container.Status)
			}
		}
		fmt.Println()
	}

	fmt.Printf("Summary: %d hosts, %d containers\n", len(topology.Hosts), totalContainers)

	return nil
}

func runStats(cmd *cobra.Command, args []string) error {
	// Initialize storage
	store, err := storage.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// Get statistics
	stats, err := store.GetStatistics()
	if err != nil {
		return fmt.Errorf("failed to get statistics: %w", err)
	}

	if queryFormat == "json" {
		return printJSON(stats)
	}

	// Print formatted stats
	fmt.Println("Infrastructure Statistics")
	fmt.Println("========================")
	fmt.Printf("\nContainers:\n")
	fmt.Printf("  Total:   %d\n", stats.TotalContainers)
	fmt.Printf("  Running: %d\n", stats.RunningContainers)
	fmt.Printf("\nHosts:\n")
	fmt.Printf("  Total:   %d\n", stats.TotalHosts)
	fmt.Printf("  With containers: %d\n", len(stats.HostContainerCounts))

	if len(stats.HostContainerCounts) > 0 {
		fmt.Printf("\nContainer Distribution:\n")
		for hostID, count := range stats.HostContainerCounts {
			fmt.Printf("  %s: %d containers\n", hostID, count)
		}
	}

	return nil
}

// printJSON prints data as formatted JSON
func printJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// printGraph prints a dependency graph as nodes and edges
func printGraph(graph *db.RelationshipGraph, level int) {
	fmt.Println("Nodes:")
	for id := range graph.Nodes {
		fmt.Printf("  - %s\n", id)
	}

	if len(graph.Edges) > 0 {
		fmt.Println("\nEdges:")
		for _, edge := range graph.Edges {
			fmt.Printf("  %s -> %s (%s)\n", edge.From, edge.To, edge.Type)
		}
	}
}
