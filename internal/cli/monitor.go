package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/pingmesh/pingmesh/internal/model"
	"github.com/spf13/cobra"
)

func newMonitorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Manage monitoring checks",
	}

	cmd.AddCommand(
		newMonitorListCmd(),
		newMonitorAddCmd(),
		newMonitorShowCmd(),
		newMonitorEditCmd(),
		newMonitorDeleteCmd(),
	)

	return cmd
}

func newMonitorListCmd() *cobra.Command {
	var group string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all monitors",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			url := fmt.Sprintf("http://%s/api/v1/monitors", cfg.CLIAddr)
			if group != "" {
				url += "?group=" + group
			}

			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			var monitors []model.Monitor
			if err := json.NewDecoder(resp.Body).Decode(&monitors); err != nil {
				return fmt.Errorf("decoding response: %w", err)
			}

			if len(monitors) == 0 {
				fmt.Println("No monitors configured.")
				return nil
			}

			fmt.Printf("%-36s  %-20s  %-12s  %-30s  %-8s\n", "ID", "NAME", "TYPE", "TARGET", "ENABLED")
			for _, m := range monitors {
				enabled := "yes"
				if !m.Enabled {
					enabled = "no"
				}
				target := m.Target
				if m.Port > 0 {
					target = fmt.Sprintf("%s:%d", m.Target, m.Port)
				}
				fmt.Printf("%-36s  %-20s  %-12s  %-30s  %-8s\n",
					m.ID, truncate(m.Name, 20), m.CheckType, truncate(target, 30), enabled)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&group, "group", "", "filter by group name")
	return cmd
}

func newMonitorAddCmd() *cobra.Command {
	var (
		name       string
		checkType  string
		target     string
		port       int
		interval   string
		timeout    string
		group      string
		keyword    string
		status     int
		dnsType    string
		dnsExpect  string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new monitor",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			if name == "" || checkType == "" || target == "" {
				return fmt.Errorf("--name, --type, and --target are required")
			}

			m := model.Monitor{
				Name:            name,
				CheckType:       model.CheckType(checkType),
				Target:          target,
				Port:            port,
				GroupName:       group,
				ExpectedKeyword: keyword,
				ExpectedStatus:  status,
				DNSRecordType:   dnsType,
				DNSExpected:     dnsExpect,
			}

			// Parse interval
			if interval != "" {
				ms, err := parseDurationMS(interval)
				if err != nil {
					return fmt.Errorf("invalid interval: %w", err)
				}
				m.IntervalMS = ms
			}

			// Parse timeout
			if timeout != "" {
				ms, err := parseDurationMS(timeout)
				if err != nil {
					return fmt.Errorf("invalid timeout: %w", err)
				}
				m.TimeoutMS = ms
			}

			body, _ := json.Marshal(m)
			resp, err := http.Post(
				fmt.Sprintf("http://%s/api/v1/monitors", cfg.CLIAddr),
				"application/json",
				bytes.NewReader(body),
			)
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				respBody, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("failed to create monitor: %s", string(respBody))
			}

			var created model.Monitor
			json.NewDecoder(resp.Body).Decode(&created)

			fmt.Printf("Monitor created: %s\n", created.ID)
			fmt.Printf("  Name:   %s\n", created.Name)
			fmt.Printf("  Type:   %s\n", created.CheckType)
			fmt.Printf("  Target: %s\n", created.Target)

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "monitor name")
	cmd.Flags().StringVar(&checkType, "type", "", "check type (icmp, tcp, http, https, dns, http_keyword)")
	cmd.Flags().StringVar(&target, "target", "", "target host")
	cmd.Flags().IntVar(&port, "port", 0, "target port")
	cmd.Flags().StringVar(&interval, "interval", "60s", "check interval")
	cmd.Flags().StringVar(&timeout, "timeout", "5s", "check timeout")
	cmd.Flags().StringVar(&group, "group", "", "monitor group name")
	cmd.Flags().StringVar(&keyword, "keyword", "", "expected keyword in response body")
	cmd.Flags().IntVar(&status, "status", 0, "expected HTTP status code")
	cmd.Flags().StringVar(&dnsType, "dns-type", "", "DNS record type (A, AAAA, CNAME)")
	cmd.Flags().StringVar(&dnsExpect, "dns-expect", "", "expected DNS answer")

	return cmd
}

func newMonitorShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show details of a monitor",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			resp, err := http.Get(fmt.Sprintf("http://%s/api/v1/monitors/%s", cfg.CLIAddr, args[0]))
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				return fmt.Errorf("monitor not found: %s", args[0])
			}

			var m model.Monitor
			json.NewDecoder(resp.Body).Decode(&m)

			fmt.Printf("ID:                %s\n", m.ID)
			fmt.Printf("Name:              %s\n", m.Name)
			fmt.Printf("Group:             %s\n", m.GroupName)
			fmt.Printf("Type:              %s\n", m.CheckType)
			fmt.Printf("Target:            %s\n", m.Target)
			if m.Port > 0 {
				fmt.Printf("Port:              %d\n", m.Port)
			}
			fmt.Printf("Interval:          %dms\n", m.IntervalMS)
			fmt.Printf("Timeout:           %dms\n", m.TimeoutMS)
			fmt.Printf("Retries:           %d\n", m.Retries)
			fmt.Printf("Failure Threshold: %d\n", m.FailureThreshold)
			fmt.Printf("Recovery Threshold:%d\n", m.RecoveryThreshold)
			fmt.Printf("Quorum:            %s\n", m.QuorumType)
			fmt.Printf("Enabled:           %v\n", m.Enabled)

			return nil
		},
	}
}

func newMonitorEditCmd() *cobra.Command {
	var (
		name    string
		target  string
		port    int
	)

	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit an existing monitor",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			updates := model.Monitor{
				Name:   name,
				Target: target,
				Port:   port,
			}

			body, _ := json.Marshal(updates)
			req, err := http.NewRequest(http.MethodPut,
				fmt.Sprintf("http://%s/api/v1/monitors/%s", cfg.CLIAddr, args[0]),
				bytes.NewReader(body))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				return fmt.Errorf("monitor not found: %s", args[0])
			}

			fmt.Println("Monitor updated.")
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "new name")
	cmd.Flags().StringVar(&target, "target", "", "new target")
	cmd.Flags().IntVar(&port, "port", 0, "new port")

	return cmd
}

func newMonitorDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a monitor",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			req, err := http.NewRequest(http.MethodDelete,
				fmt.Sprintf("http://%s/api/v1/monitors/%s", cfg.CLIAddr, args[0]), nil)
			if err != nil {
				return err
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			fmt.Println("Monitor deleted.")
			return nil
		},
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func parseDurationMS(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "ms") {
		s = strings.TrimSuffix(s, "ms")
		var ms int64
		_, err := fmt.Sscanf(s, "%d", &ms)
		return ms, err
	}
	if strings.HasSuffix(s, "s") {
		s = strings.TrimSuffix(s, "s")
		var sec int64
		_, err := fmt.Sscanf(s, "%d", &sec)
		return sec * 1000, err
	}
	if strings.HasSuffix(s, "m") {
		s = strings.TrimSuffix(s, "m")
		var min int64
		_, err := fmt.Sscanf(s, "%d", &min)
		return min * 60000, err
	}
	var ms int64
	_, err := fmt.Sscanf(s, "%d", &ms)
	return ms, err
}
