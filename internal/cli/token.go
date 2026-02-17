package cli

import (
	"fmt"
	"time"

	"github.com/pingmesh/pingmesh/internal/cluster"
	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/pingmesh/pingmesh/internal/store"
	"github.com/spf13/cobra"
)

func newJoinTokenCmd() *cobra.Command {
	var expires string

	cmd := &cobra.Command{
		Use:   "join-token",
		Short: "Generate a one-time join token",
		Long:  "Generate a token that allows a new node to join the cluster. Tokens are single-use and expire.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			if cfg.Role != "coordinator" {
				return fmt.Errorf("join tokens can only be generated on the coordinator")
			}

			expiry, err := time.ParseDuration(expires)
			if err != nil {
				return fmt.Errorf("invalid expiry duration: %w", err)
			}

			st, err := store.NewSQLiteStore(cfg.DBPath())
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer st.Close()

			token, err := cluster.GenerateJoinToken(st, cfg.ListenAddr, expiry)
			if err != nil {
				return fmt.Errorf("generating token: %w", err)
			}

			fmt.Println("Join token generated (single-use, expires in", expires+")")
			fmt.Println()
			fmt.Println("Run this on the new node:")
			fmt.Printf("  pingmesh join %s\n", token)
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().StringVar(&expires, "expires", "1h", "token expiry duration")

	return cmd
}
