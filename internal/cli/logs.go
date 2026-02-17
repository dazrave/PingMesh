package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/spf13/cobra"
)

type logEntry struct {
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
}

func newLogsCmd() *cobra.Command {
	var lines int

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show recent agent log entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			url := fmt.Sprintf("http://%s/api/v1/logs?lines=%d", cfg.CLIAddr, lines)
			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			var entries []logEntry
			if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
				return fmt.Errorf("decoding response: %w", err)
			}

			for _, e := range entries {
				fmt.Printf("%s  %s\n", e.Time.Format("15:04:05.000"), e.Message)
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&lines, "lines", "n", 100, "number of log lines to show")
	return cmd
}
