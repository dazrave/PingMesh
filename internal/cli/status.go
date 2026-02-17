package cli

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/pingmesh/pingmesh/internal/model"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show cluster status overview",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			resp, err := http.Get(fmt.Sprintf("http://%s/api/v1/status", cfg.CLIAddr))
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			var status model.ClusterStatus
			if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
				return fmt.Errorf("decoding response: %w", err)
			}

			fmt.Printf("Node:     %s (%s)\n", status.NodeID[:8], status.Role)
			fmt.Printf("Nodes:    %d\n", len(status.Nodes))
			fmt.Printf("Monitors: %d\n", status.MonitorCount)
			fmt.Printf("Active Incidents: %d\n", len(status.ActiveIncidents))

			if len(status.Nodes) > 0 {
				fmt.Println()
				fmt.Println("Nodes:")
				for _, n := range status.Nodes {
					fmt.Printf("  %s  %-15s  %-12s  %s\n", n.ID[:8], n.Name, n.Role, n.Status)
				}
			}

			if len(status.ActiveIncidents) > 0 {
				fmt.Println()
				fmt.Println("Active Incidents:")
				for _, inc := range status.ActiveIncidents {
					fmt.Printf("  %s  monitor=%s  status=%s\n", inc.ID[:8], inc.MonitorID[:8], inc.Status)
				}
			}

			return nil
		},
	}
}
