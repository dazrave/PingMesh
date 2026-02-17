package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/pingmesh/pingmesh/internal/model"
	"github.com/spf13/cobra"
)

func newIncidentsCmd() *cobra.Command {
	var activeOnly bool

	cmd := &cobra.Command{
		Use:   "incidents",
		Short: "List incidents",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			url := fmt.Sprintf("http://%s/api/v1/incidents", cfg.CLIAddr)
			if activeOnly {
				url += "?active=true"
			}

			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			var incidents []model.Incident
			if err := json.NewDecoder(resp.Body).Decode(&incidents); err != nil {
				return fmt.Errorf("decoding response: %w", err)
			}

			if len(incidents) == 0 {
				fmt.Println("No incidents found.")
				return nil
			}

			fmt.Printf("%-10s  %-10s  %-12s  %-20s  %s\n", "ID", "MONITOR", "STATUS", "STARTED", "CONFIRMED BY")
			for _, inc := range incidents {
				started := time.UnixMilli(inc.StartedAt).Format("2006-01-02 15:04")
				confirmedBy := "-"
				if len(inc.ConfirmingNodes) > 0 {
					confirmedBy = fmt.Sprintf("%d nodes", len(inc.ConfirmingNodes))
				}
				fmt.Printf("%-10s  %-10s  %-12s  %-20s  %s\n",
					inc.ID[:8], inc.MonitorID[:8], inc.Status, started, confirmedBy)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&activeOnly, "active", false, "show only active incidents")
	return cmd
}
