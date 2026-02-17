package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/pingmesh/pingmesh/internal/model"
	"github.com/spf13/cobra"
)

func newNodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Manage cluster nodes",
	}

	cmd.AddCommand(
		newNodeListCmd(),
		newNodeShowCmd(),
		newNodeRemoveCmd(),
	)

	return cmd
}

func newNodeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all nodes in the cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			resp, err := http.Get(fmt.Sprintf("http://%s/api/v1/nodes", cfg.CLIAddr))
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			var nodes []model.Node
			if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
				return fmt.Errorf("decoding response: %w", err)
			}

			if len(nodes) == 0 {
				fmt.Println("No nodes found.")
				return nil
			}

			fmt.Printf("%-36s  %-15s  %-12s  %-8s  %s\n", "ID", "NAME", "ROLE", "STATUS", "ADDRESS")
			for _, n := range nodes {
				fmt.Printf("%-36s  %-15s  %-12s  %-8s  %s\n", n.ID, n.Name, n.Role, n.Status, n.Address)
			}

			return nil
		},
	}
}

func newNodeShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show details of a specific node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			resp, err := http.Get(fmt.Sprintf("http://%s/api/v1/nodes/%s", cfg.CLIAddr, args[0]))
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				return fmt.Errorf("node not found: %s", args[0])
			}

			body, _ := io.ReadAll(resp.Body)
			var node model.Node
			json.Unmarshal(body, &node)

			fmt.Printf("ID:        %s\n", node.ID)
			fmt.Printf("Name:      %s\n", node.Name)
			fmt.Printf("Location:  %s\n", node.Location)
			fmt.Printf("Address:   %s\n", node.Address)
			fmt.Printf("Role:      %s\n", node.Role)
			fmt.Printf("Status:    %s\n", node.Status)

			return nil
		},
	}
}

func newNodeRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a node from the cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://%s/api/v1/nodes/%s", cfg.CLIAddr, args[0]), nil)
			if err != nil {
				return err
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			fmt.Println("Node removed.")
			return nil
		},
	}
}
