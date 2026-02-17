package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/pingmesh/pingmesh/internal/model"
	"github.com/spf13/cobra"
)

func newTestPeerCmd() *cobra.Command {
	var nodeID string

	cmd := &cobra.Command{
		Use:   "test-peer",
		Short: "Test connectivity to peer nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			u := fmt.Sprintf("http://%s/api/v1/test-peer", cfg.CLIAddr)
			if nodeID != "" {
				u += "?node=" + url.QueryEscape(nodeID)
			}

			resp, err := http.Get(u)
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			var peers []model.PeerStatus
			if err := json.NewDecoder(resp.Body).Decode(&peers); err != nil {
				return fmt.Errorf("decoding response: %w", err)
			}

			if len(peers) == 0 {
				fmt.Println("No peers found.")
				return nil
			}

			fmt.Printf("%-20s %-25s %-10s %-10s %s\n", "NAME", "ADDRESS", "STATUS", "REACH", "LATENCY")
			for _, p := range peers {
				reach := "no"
				latency := "-"
				if p.Reachable {
					reach = "yes"
					latency = fmt.Sprintf("%.1fms", p.LatencyMS)
				}
				errStr := ""
				if p.Error != "" {
					errStr = "  (" + p.Error + ")"
				}
				fmt.Printf("%-20s %-25s %-10s %-10s %s%s\n", p.Name, p.Address, p.Status, reach, latency, errStr)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&nodeID, "node", "", "test only a specific node ID")
	return cmd
}
