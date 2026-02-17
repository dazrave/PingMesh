package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/pingmesh/pingmesh/internal/model"
	"github.com/spf13/cobra"
)

func newAlertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alert",
		Short: "Manage alert channels",
	}

	cmd.AddCommand(
		newAlertListCmd(),
		newAlertAddWebhookCmd(),
		newAlertAddEmailCmd(),
		newAlertShowCmd(),
		newAlertDeleteCmd(),
		newAlertTestCmd(),
		newAlertHistoryCmd(),
	)

	return cmd
}

func newAlertListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all alert channels",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			resp, err := http.Get(fmt.Sprintf("http://%s/api/v1/alerts/channels", cfg.CLIAddr))
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			var channels []model.AlertChannel
			if err := json.NewDecoder(resp.Body).Decode(&channels); err != nil {
				return fmt.Errorf("decoding response: %w", err)
			}

			if len(channels) == 0 {
				fmt.Println("No alert channels configured.")
				return nil
			}

			fmt.Printf("%-36s  %-20s  %-10s  %-8s\n", "ID", "NAME", "TYPE", "ENABLED")
			for _, ch := range channels {
				enabled := "yes"
				if !ch.Enabled {
					enabled = "no"
				}
				fmt.Printf("%-36s  %-20s  %-10s  %-8s\n",
					ch.ID, truncate(ch.Name, 20), ch.Type, enabled)
			}

			return nil
		},
	}
}

func newAlertAddWebhookCmd() *cobra.Command {
	var (
		name   string
		url    string
		secret string
	)

	cmd := &cobra.Command{
		Use:   "add-webhook",
		Short: "Add a webhook alert channel",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			if name == "" || url == "" {
				return fmt.Errorf("--name and --url are required")
			}

			whCfg := model.WebhookConfig{
				URL:    url,
				Secret: secret,
			}
			cfgJSON, _ := json.Marshal(whCfg)

			ch := model.AlertChannel{
				Name:   name,
				Type:   "webhook",
				Config: string(cfgJSON),
			}

			body, _ := json.Marshal(ch)
			resp, err := http.Post(
				fmt.Sprintf("http://%s/api/v1/alerts/channels", cfg.CLIAddr),
				"application/json",
				bytes.NewReader(body),
			)
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				respBody, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("failed to create channel: %s", string(respBody))
			}

			var created model.AlertChannel
			json.NewDecoder(resp.Body).Decode(&created)

			fmt.Printf("Webhook channel created: %s\n", created.ID)
			fmt.Printf("  Name: %s\n", created.Name)
			fmt.Printf("  URL:  %s\n", url)

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "channel name")
	cmd.Flags().StringVar(&url, "url", "", "webhook URL")
	cmd.Flags().StringVar(&secret, "secret", "", "HMAC-SHA256 signing secret (optional)")

	return cmd
}

func newAlertAddEmailCmd() *cobra.Command {
	var (
		name     string
		to       string
		smtpHost string
		smtpPort int
		username string
		password string
		from     string
		useTLS   bool
	)

	cmd := &cobra.Command{
		Use:   "add-email",
		Short: "Add an email alert channel",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			if name == "" || to == "" || smtpHost == "" || from == "" {
				return fmt.Errorf("--name, --to, --smtp-host, and --from are required")
			}

			emailCfg := model.EmailConfig{
				SMTPHost: smtpHost,
				SMTPPort: smtpPort,
				Username: username,
				Password: password,
				From:     from,
				To:       to,
				TLS:      useTLS,
			}
			cfgJSON, _ := json.Marshal(emailCfg)

			ch := model.AlertChannel{
				Name:   name,
				Type:   "email",
				Config: string(cfgJSON),
			}

			body, _ := json.Marshal(ch)
			resp, err := http.Post(
				fmt.Sprintf("http://%s/api/v1/alerts/channels", cfg.CLIAddr),
				"application/json",
				bytes.NewReader(body),
			)
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				respBody, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("failed to create channel: %s", string(respBody))
			}

			var created model.AlertChannel
			json.NewDecoder(resp.Body).Decode(&created)

			fmt.Printf("Email channel created: %s\n", created.ID)
			fmt.Printf("  Name: %s\n", created.Name)
			fmt.Printf("  To:   %s\n", to)
			fmt.Printf("  SMTP: %s:%d\n", smtpHost, smtpPort)

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "channel name")
	cmd.Flags().StringVar(&to, "to", "", "recipient email address")
	cmd.Flags().StringVar(&smtpHost, "smtp-host", "", "SMTP server hostname")
	cmd.Flags().IntVar(&smtpPort, "smtp-port", 587, "SMTP server port")
	cmd.Flags().StringVar(&username, "username", "", "SMTP username")
	cmd.Flags().StringVar(&password, "password", "", "SMTP password")
	cmd.Flags().StringVar(&from, "from", "", "sender email address")
	cmd.Flags().BoolVar(&useTLS, "tls", false, "use TLS for SMTP connection")

	return cmd
}

func newAlertShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show details of an alert channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			resp, err := http.Get(fmt.Sprintf("http://%s/api/v1/alerts/channels/%s", cfg.CLIAddr, args[0]))
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				return fmt.Errorf("alert channel not found: %s", args[0])
			}

			var ch model.AlertChannel
			json.NewDecoder(resp.Body).Decode(&ch)

			enabled := "yes"
			if !ch.Enabled {
				enabled = "no"
			}

			fmt.Printf("ID:        %s\n", ch.ID)
			fmt.Printf("Name:      %s\n", ch.Name)
			fmt.Printf("Type:      %s\n", ch.Type)
			fmt.Printf("Enabled:   %s\n", enabled)
			fmt.Printf("Created:   %s\n", time.UnixMilli(ch.CreatedAt).Format(time.RFC3339))

			switch ch.Type {
			case "webhook":
				var whCfg model.WebhookConfig
				json.Unmarshal([]byte(ch.Config), &whCfg)
				fmt.Printf("URL:       %s\n", whCfg.URL)
				if whCfg.Secret != "" {
					fmt.Printf("Secret:    (configured)\n")
				}
			case "email":
				var emailCfg model.EmailConfig
				json.Unmarshal([]byte(ch.Config), &emailCfg)
				fmt.Printf("To:        %s\n", emailCfg.To)
				fmt.Printf("From:      %s\n", emailCfg.From)
				fmt.Printf("SMTP:      %s:%d\n", emailCfg.SMTPHost, emailCfg.SMTPPort)
				fmt.Printf("TLS:       %v\n", emailCfg.TLS)
			}

			return nil
		},
	}
}

func newAlertDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an alert channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			req, err := http.NewRequest(http.MethodDelete,
				fmt.Sprintf("http://%s/api/v1/alerts/channels/%s", cfg.CLIAddr, args[0]), nil)
			if err != nil {
				return err
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			fmt.Println("Alert channel deleted.")
			return nil
		},
	}
}

func newAlertTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test <id>",
		Short: "Send a test alert to a channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			resp, err := http.Post(
				fmt.Sprintf("http://%s/api/v1/alerts/channels/%s/test", cfg.CLIAddr, args[0]),
				"application/json",
				nil,
			)
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				respBody, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("test failed: %s", string(respBody))
			}

			fmt.Println("Test alert sent successfully.")
			return nil
		},
	}
}

func newAlertHistoryCmd() *cobra.Command {
	var (
		channelID string
		limit     int
	)

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show alert delivery history",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dataDir)
			if err != nil {
				return err
			}

			url := fmt.Sprintf("http://%s/api/v1/alerts/history?limit=%d", cfg.CLIAddr, limit)
			if channelID != "" {
				url += "&channel=" + channelID
			}

			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("connecting to agent: %w (is the agent running?)", err)
			}
			defer resp.Body.Close()

			var records []model.AlertRecord
			if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
				return fmt.Errorf("decoding response: %w", err)
			}

			if len(records) == 0 {
				fmt.Println("No alert history.")
				return nil
			}

			fmt.Printf("%-20s  %-10s  %-10s  %-10s  %-8s  %s\n", "TIME", "CHANNEL", "INCIDENT", "EVENT", "STATUS", "ERROR")
			for _, rec := range records {
				ts := time.UnixMilli(rec.SentAt).Format("15:04:05")
				chID := rec.ChannelID
				if len(chID) > 8 {
					chID = chID[:8]
				}
				incID := rec.IncidentID
				if len(incID) > 8 {
					incID = incID[:8]
				}
				errStr := rec.Error
				if len(errStr) > 40 {
					errStr = errStr[:37] + "..."
				}
				fmt.Printf("%-20s  %-10s  %-10s  %-10s  %-8s  %s\n",
					ts, chID, incID, rec.EventType, rec.Status, errStr)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&channelID, "channel", "", "filter by channel ID")
	cmd.Flags().IntVar(&limit, "limit", 50, "max records to show")

	return cmd
}
