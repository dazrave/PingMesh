package cli

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/pingmesh/pingmesh/internal/agent"
	"github.com/pingmesh/pingmesh/internal/api"
	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/pingmesh/pingmesh/internal/logbuf"
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

			// Set up in-memory log ring buffer
			logBuf := logbuf.New()
			log.SetOutput(io.MultiWriter(os.Stdout, logBuf))

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

			// Create agent first so we can pass it as AgentInfo
			a := agent.New(cfg, st)

			// Start API server with log buffer, agent info, and alert dispatcher
			apiServer := api.NewServer(cfg, st,
				api.WithLogBuffer(logBuf),
				api.WithAgentInfo(a),
				api.WithAlertDispatcher(a.Alerter()),
			)
			go func() {
				if err := apiServer.StartCLI(ctx); err != nil {
					log.Printf("[api] CLI server error: %v", err)
				}
			}()

			// Start peer API server
			go func() {
				if err := apiServer.StartPeer(ctx); err != nil {
					log.Printf("[api] Peer server error: %v", err)
				}
			}()

			// Start agent
			return a.Run(ctx)
		},
	}
}
