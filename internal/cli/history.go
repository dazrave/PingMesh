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

func newHistoryCmd() *cobra.Command {
	var (
		monitorID string
		nodeID    string
		since     string
		limit     int
	)

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show check result history",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			url := fmt.Sprintf("http://%s/api/v1/history?limit=%d", cfg.CLIAddr, limit)
			if monitorID != "" {
				url += "&monitor=" + monitorID
			}
			if nodeID != "" {
				url += "&node=" + nodeID
			}
			if since != "" {
				d, err := time.ParseDuration(since)
				if err != nil {
					return fmt.Errorf("invalid --since duration: %w", err)
				}
				sinceMS := time.Now().Add(-d).UnixMilli()
				url += fmt.Sprintf("&since=%d", sinceMS)
			}

			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			var results []model.CheckResult
			if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
				return fmt.Errorf("decoding response: %w", err)
			}

			if len(results) == 0 {
				fmt.Println("No results found.")
				return nil
			}

			fmt.Printf("%-20s  %-10s  %-10s  %-8s  %-10s  %s\n", "TIME", "MONITOR", "NODE", "STATUS", "LATENCY", "ERROR")
			for _, r := range results {
				ts := time.UnixMilli(r.Timestamp).Format("15:04:05")
				monID := r.MonitorID
				if len(monID) > 8 {
					monID = monID[:8]
				}
				nID := r.NodeID
				if len(nID) > 8 {
					nID = nID[:8]
				}
				errStr := r.Error
				if len(errStr) > 40 {
					errStr = errStr[:37] + "..."
				}
				fmt.Printf("%-20s  %-10s  %-10s  %-8s  %7.1fms  %s\n",
					ts, monID, nID, r.Status, r.LatencyMS, errStr)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&monitorID, "monitor", "", "filter by monitor ID")
	cmd.Flags().StringVar(&nodeID, "node", "", "filter by node ID")
	cmd.Flags().StringVar(&since, "since", "24h", "show results since duration ago")
	cmd.Flags().IntVar(&limit, "limit", 50, "max results to show")

	return cmd
}
