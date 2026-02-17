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

			fmt.Printf("Node ID:      %s\n", health.NodeID)
			fmt.Printf("Name:         %s\n", health.Name)
			fmt.Printf("Role:         %s\n", health.Role)
			fmt.Printf("Uptime:       %s\n", health.Uptime)
			fmt.Printf("Go Version:   %s\n", health.GoVersion)
			fmt.Printf("Goroutines:   %d\n", health.NumGoroutines)
			fmt.Printf("Memory:       %.1f MB\n", health.MemoryMB)
			fmt.Printf("DB Size:      %.1f MB\n", health.DBSizeMB)

			return nil
		},
	}
}
