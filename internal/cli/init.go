package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/pingmesh/pingmesh/internal/cluster"
	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/pingmesh/pingmesh/internal/model"
	"github.com/pingmesh/pingmesh/internal/store"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var listenAddr string
	var nodeName string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize this node as a coordinator",
		Long:  "Initialize PingMesh on this node, creating the internal CA and setting it up as the cluster coordinator.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if already initialized
			if _, err := config.Load(dataDir); err == nil {
				return fmt.Errorf("already initialized (config exists at %s)", dataDir)
			}

			if nodeName == "" {
				hostname, _ := os.Hostname()
				nodeName = hostname
			}

			nodeID := uuid.New().String()

			cfg := &config.Config{
				NodeID:     nodeID,
				NodeName:   nodeName,
				Role:       model.RoleCoordinator,
				DataDir:    dataDir,
				ListenAddr: listenAddr,
				CLIAddr:    config.DefaultCLIAddr,
				TLS: &config.TLSConfig{
					CAPath:   "certs/ca.crt",
					CertPath: "certs/node.crt",
					KeyPath:  "certs/node.key",
				},
			}

			// Create data directory
			if err := os.MkdirAll(dataDir, 0750); err != nil {
				return fmt.Errorf("creating data directory: %w", err)
			}

			// Generate CA
			certsDir := cfg.CertsDir()
			fmt.Println("Generating internal CA...")
			if err := cluster.GenerateCA(certsDir); err != nil {
				return fmt.Errorf("generating CA: %w", err)
			}

			// Generate coordinator cert
			fmt.Println("Generating coordinator certificate...")
			if err := cluster.GenerateNodeCert(certsDir, nodeID, []string{"127.0.0.1", "0.0.0.0"}); err != nil {
				return fmt.Errorf("generating node cert: %w", err)
			}

			// Save config
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			// Initialize database
			st, err := store.NewSQLiteStore(cfg.DBPath())
			if err != nil {
				return fmt.Errorf("initializing database: %w", err)
			}
			defer st.Close()

			// Register this node
			now := time.Now().UnixMilli()
			node := &model.Node{
				ID:        nodeID,
				Name:      nodeName,
				Address:   listenAddr,
				Role:      model.RoleCoordinator,
				Status:    model.NodeOnline,
				LastSeen:  now,
				CreatedAt: now,
			}
			if err := st.CreateNode(node); err != nil {
				return fmt.Errorf("registering node: %w", err)
			}

			fmt.Println()
			fmt.Printf("PingMesh initialized successfully!\n")
			fmt.Printf("  Node ID:    %s\n", nodeID)
			fmt.Printf("  Node Name:  %s\n", nodeName)
			fmt.Printf("  Role:       coordinator\n")
			fmt.Printf("  Data Dir:   %s\n", dataDir)
			fmt.Printf("  Listen:     %s\n", listenAddr)
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Println("  1. Start the agent:    pingmesh agent")
			fmt.Println("  2. Add a monitor:      pingmesh monitor add --name 'My Site' --type http --target example.com")
			fmt.Println("  3. Generate join token: pingmesh join-token")

			return nil
		},
	}

	cmd.Flags().StringVar(&listenAddr, "listen", config.DefaultListenAddr, "listen address for peer API")
	cmd.Flags().StringVar(&nodeName, "name", "", "node name (defaults to hostname)")

	return cmd
}
