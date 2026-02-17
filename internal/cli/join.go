package cli

import (
	"fmt"

	"github.com/pingmesh/pingmesh/internal/cluster"
	"github.com/spf13/cobra"
)

func newJoinCmd() *cobra.Command {
	var nodeName string

	cmd := &cobra.Command{
		Use:   "join <token>",
		Short: "Join an existing PingMesh cluster",
		Long:  "Join this node to an existing PingMesh cluster using a one-time join token.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tokenStr := args[0]

			token, err := cluster.DecodeJoinToken(tokenStr)
			if err != nil {
				return fmt.Errorf("invalid token: %w", err)
			}

			fmt.Printf("Coordinator: %s\n", token.CoordinatorAddr)
			fmt.Printf("Token expires: %s\n", token.ExpiresAt)

			// TODO (M3): Connect to coordinator, present secret, receive certs, save config
			fmt.Println()
			fmt.Println("Join flow not yet implemented (M3 milestone).")
			fmt.Println("The node would connect to the coordinator, validate the token,")
			fmt.Println("receive its certificate, and begin operating as a cluster member.")

			return nil
		},
	}

	cmd.Flags().StringVar(&nodeName, "name", "", "node name (defaults to hostname)")

	return cmd
}
