package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var dataDir string

// NewRootCmd creates the root cobra command.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "pingmesh",
		Short: "Distributed multi-vantage monitoring system",
		Long:  "PingMesh is a distributed monitoring system where multiple lightweight nodes run checks from different networks and confirm failures with each other before alerting.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&dataDir, "data-dir", "/var/lib/pingmesh", "data directory path")

	root.AddCommand(
		newInitCmd(),
		newJoinCmd(),
		newJoinTokenCmd(),
		newNodeCmd(),
		newMonitorCmd(),
		newStatusCmd(),
		newIncidentsCmd(),
		newHistoryCmd(),
		newHealthCmd(),
		newLogsCmd(),
		newTestPeerCmd(),
		newAlertCmd(),
		newAgentCmd(),
	)

	return root
}

// Execute runs the root command.
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
