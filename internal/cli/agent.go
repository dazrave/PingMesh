package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/pingmesh/pingmesh/internal/agent"
	"github.com/pingmesh/pingmesh/internal/api"
	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/pingmesh/pingmesh/internal/store"
	"github.com/spf13/cobra"
)

func newAgentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agent",
		Short: "Run the PingMesh agent",
		Long:  "Start the PingMesh agent daemon that runs monitoring checks and serves the API.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			st, err := store.NewSQLiteStore(cfg.DBPath())
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer st.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Handle shutdown signals
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				sig := <-sigCh
				log.Printf("[agent] received signal %v, shutting down...", sig)
				cancel()
			}()

			// Start API server
			apiServer := api.NewServer(cfg, st)
			go func() {
				if err := apiServer.StartCLI(ctx); err != nil {
					log.Printf("[api] CLI server error: %v", err)
				}
			}()

			// Start agent
			a := agent.New(cfg, st)
			return a.Run(ctx)
		},
	}
}
