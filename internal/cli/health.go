package cli

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/pingmesh/pingmesh/internal/model"
	"github.com/spf13/cobra"
)

func newHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Show local node health",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			resp, err := http.Get(fmt.Sprintf("http://%s/api/v1/health", cfg.CLIAddr))
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			var health model.HealthInfo
			if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
				return fmt.Errorf("decoding response: %w", err)
			}

			fmt.Printf("Node ID:         %s\n", health.NodeID)
			fmt.Printf("Name:            %s\n", health.Name)
			fmt.Printf("Role:            %s\n", health.Role)
			fmt.Printf("Uptime:          %s\n", health.Uptime)
			fmt.Printf("Go Version:      %s\n", health.GoVersion)
			fmt.Printf("Goroutines:      %d\n", health.NumGoroutines)
			fmt.Printf("Memory:          %.1f MB\n", health.MemoryMB)
			fmt.Printf("DB Size:         %.1f MB\n", health.DBSizeMB)
			fmt.Printf("Active Monitors: %d\n", health.ActiveMonitors)
			if health.LastHeartbeat != "" {
				fmt.Printf("Last Heartbeat:  %s\n", health.LastHeartbeat)
			}
			if health.LastConfigSync != "" {
				fmt.Printf("Last Config Sync:%s\n", health.LastConfigSync)
			}
			if health.Coordinator != "" {
				fmt.Printf("Coordinator:     %s\n", health.Coordinator)
			}

			if len(health.Peers) > 0 {
				fmt.Println()
				fmt.Printf("%-20s %-25s %-10s %-10s %s\n", "PEER", "ADDRESS", "STATUS", "REACH", "LATENCY")
				for _, p := range health.Peers {
					reach := "no"
					latency := "-"
					if p.Reachable {
						reach = "yes"
						latency = fmt.Sprintf("%.1fms", p.LatencyMS)
					}
					fmt.Printf("%-20s %-25s %-10s %-10s %s\n", p.Name, p.Address, p.Status, reach, latency)
				}
			}

			return nil
		},
	}
}
