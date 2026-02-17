package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pingmesh/pingmesh/internal/cluster"
	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/pingmesh/pingmesh/internal/model"
	"github.com/pingmesh/pingmesh/internal/store"
	"github.com/spf13/cobra"
)

func newJoinCmd() *cobra.Command {
	var (
		nodeName   string
		listenAddr string
		cliAddr    string
	)

	cmd := &cobra.Command{
		Use:   "join <token>",
		Short: "Join an existing PingMesh cluster",
		Long:  "Join this node to an existing PingMesh cluster using a one-time join token.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tokenStr := args[0]

			// Check if already initialized
			if _, err := config.Load(dataDir); err == nil {
				return fmt.Errorf("already initialized (config exists at %s)", dataDir)
			}

			// Decode token to get coordinator address
			token, err := cluster.DecodeJoinToken(tokenStr)
			if err != nil {
				return fmt.Errorf("invalid token: %w", err)
			}

			fmt.Printf("Coordinator: %s\n", token.CoordinatorAddr)
			fmt.Printf("Token expires: %s\n", token.ExpiresAt.Format(time.RFC3339))

			// Resolve node name
			if nodeName == "" {
				hostname, _ := os.Hostname()
				nodeName = hostname
			}

			// Send join request
			fmt.Println("Joining cluster...")
			client := cluster.NewPeerClient()
			joinReq := &model.JoinRequest{
				Secret:     token.Secret,
				Name:       nodeName,
				ListenAddr: listenAddr,
				CLIAddr:    cliAddr,
			}

			resp, err := client.Join(token.CoordinatorAddr, joinReq)
			if err != nil {
				return fmt.Errorf("join failed: %w", err)
			}

			// Create data dir and certs dir
			certsDir := filepath.Join(dataDir, "certs")
			if err := os.MkdirAll(certsDir, 0700); err != nil {
				return fmt.Errorf("creating certs directory: %w", err)
			}

			// Write certificates from response
			if err := os.WriteFile(filepath.Join(certsDir, "ca.crt"), []byte(resp.CACert), 0644); err != nil {
				return fmt.Errorf("writing CA cert: %w", err)
			}
			if err := os.WriteFile(filepath.Join(certsDir, "node.crt"), []byte(resp.NodeCert), 0644); err != nil {
				return fmt.Errorf("writing node cert: %w", err)
			}
			if err := os.WriteFile(filepath.Join(certsDir, "node.key"), []byte(resp.NodeKey), 0600); err != nil {
				return fmt.Errorf("writing node key: %w", err)
			}

			// Build and save config
			cfg := &config.Config{
				NodeID:     resp.NodeID,
				NodeName:   nodeName,
				Role:       model.RoleNode,
				DataDir:    dataDir,
				ListenAddr: listenAddr,
				CLIAddr:    cliAddr,
				Coordinator: &config.CoordinatorConfig{
					Address: token.CoordinatorAddr,
				},
				TLS: &config.TLSConfig{
					CAPath:   "certs/ca.crt",
					CertPath: "certs/node.crt",
					KeyPath:  "certs/node.key",
				},
			}

			if err := cfg.Save(); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			// Initialize database and register self
			st, err := store.NewSQLiteStore(cfg.DBPath())
			if err != nil {
				return fmt.Errorf("initializing database: %w", err)
			}
			defer st.Close()

			now := time.Now().UnixMilli()
			node := &model.Node{
				ID:        resp.NodeID,
				Name:      nodeName,
				Address:   listenAddr,
				Role:      model.RoleNode,
				Status:    model.NodeOnline,
				LastSeen:  now,
				CreatedAt: now,
			}
			if err := st.CreateNode(node); err != nil {
				return fmt.Errorf("registering node: %w", err)
			}

			fmt.Println()
			fmt.Printf("Joined cluster successfully!\n")
			fmt.Printf("  Node ID:      %s\n", resp.NodeID)
			fmt.Printf("  Node Name:    %s\n", nodeName)
			fmt.Printf("  Role:         node\n")
			fmt.Printf("  Data Dir:     %s\n", dataDir)
			fmt.Printf("  Listen:       %s\n", listenAddr)
			fmt.Printf("  CLI:          %s\n", cliAddr)
			fmt.Printf("  Coordinator:  %s\n", token.CoordinatorAddr)
			fmt.Println()
			fmt.Println("Next: run `pingmesh agent` to start this node.")

			return nil
		},
	}

	cmd.Flags().StringVar(&nodeName, "name", "", "node name (defaults to hostname)")
	cmd.Flags().StringVar(&listenAddr, "listen", config.DefaultListenAddr, "listen address for peer API")
	cmd.Flags().StringVar(&cliAddr, "cli-addr", config.DefaultCLIAddr, "listen address for CLI API")

	return cmd
}
