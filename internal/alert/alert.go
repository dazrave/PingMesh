package alert

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"time"

	"github.com/pingmesh/pingmesh/internal/model"
	"github.com/pingmesh/pingmesh/internal/store"
)

// Dispatcher handles sending alerts through configured channels.
type Dispatcher struct {
	store store.Store
}

// NewDispatcher creates a new alert dispatcher backed by the given store.
func NewDispatcher(st store.Store) *Dispatcher {
	return &Dispatcher{store: st}
}

// SendAlert sends an alert for a confirmed incident to all enabled channels.
func (d *Dispatcher) SendAlert(incident *model.Incident, monitor *model.Monitor) {
	log.Printf("[ALERT] INCIDENT CONFIRMED: monitor=%s (%s) target=%s incident=%s confirming_nodes=%v",
		monitor.Name, monitor.CheckType, monitor.Target, incident.ID, incident.ConfirmingNodes)

	d.dispatch(incident, monitor, "alert")
}

// SendRecovery sends a recovery notification to all enabled channels.
func (d *Dispatcher) SendRecovery(incident *model.Incident, monitor *model.Monitor) {
	log.Printf("[ALERT] INCIDENT RESOLVED: monitor=%s (%s) target=%s incident=%s",
		monitor.Name, monitor.CheckType, monitor.Target, incident.ID)

	d.dispatch(incident, monitor, "recovery")
}

// SendTest sends a test alert to a specific channel.
func (d *Dispatcher) SendTest(channelID string) error {
	ch, err := d.store.GetAlertChannel(channelID)
	if err != nil {
		return fmt.Errorf("loading channel: %w", err)
	}
	if ch == nil {
		return fmt.Errorf("channel not found: %s", channelID)
	}

	now := time.Now()
	incident := &model.Incident{
		ID:              "test-" + now.Format("20060102-150405"),
		MonitorID:       "test-monitor",
		Status:          model.IncidentConfirmed,
		StartedAt:       now.Add(-2 * time.Minute).UnixMilli(),
		ConfirmedAt:     now.UnixMilli(),
		ConfirmingNodes: []string{"test-node-1", "test-node-2"},
	}
	monitor := &model.Monitor{
		ID:        "test-monitor",
		Name:      "Test Monitor",
		CheckType: model.CheckHTTP,
		Target:    "example.com",
		GroupName: "test",
	}

	sendErr := d.sendToChannel(ch, incident, monitor, "alert")

	// Record in history
	rec := &model.AlertRecord{
		ChannelID:  ch.ID,
		IncidentID: incident.ID,
		MonitorID:  monitor.ID,
		EventType:  "test",
		Status:     "success",
		SentAt:     now.UnixMilli(),
	}
	if sendErr != nil {
		rec.Status = "failed"
		rec.Error = sendErr.Error()
	}
	if err := d.store.InsertAlertRecord(rec); err != nil {
		log.Printf("[alert] failed to record test alert history: %v", err)
	}

	return sendErr
}

func (d *Dispatcher) dispatch(incident *model.Incident, monitor *model.Monitor, eventType string) {
	channels, err := d.store.ListEnabledAlertChannels()
	if err != nil {
		log.Printf("[alert] error loading channels: %v", err)
		return
	}
	if len(channels) == 0 {
		return
	}

	for i := range channels {
		ch := channels[i]
		go func() {
			sendErr := d.sendToChannel(&ch, incident, monitor, eventType)

			rec := &model.AlertRecord{
				ChannelID:  ch.ID,
				IncidentID: incident.ID,
				MonitorID:  monitor.ID,
				EventType:  eventType,
				Status:     "success",
				SentAt:     time.Now().UnixMilli(),
			}
			if sendErr != nil {
				rec.Status = "failed"
				rec.Error = sendErr.Error()
				log.Printf("[alert] send to channel %s (%s) failed: %v", ch.Name, ch.Type, sendErr)
			} else {
				log.Printf("[alert] sent %s to channel %s (%s)", eventType, ch.Name, ch.Type)
			}

			if err := d.store.InsertAlertRecord(rec); err != nil {
				log.Printf("[alert] failed to record alert history: %v", err)
			}
		}()
	}
}

func (d *Dispatcher) sendToChannel(ch *model.AlertChannel, incident *model.Incident, monitor *model.Monitor, eventType string) error {
	switch ch.Type {
	case "webhook":
		return d.sendWebhook(ch, incident, monitor, eventType)
	case "email":
		return d.sendEmail(ch, incident, monitor, eventType)
	default:
		return fmt.Errorf("unknown channel type: %s", ch.Type)
	}
}

func (d *Dispatcher) sendWebhook(ch *model.AlertChannel, incident *model.Incident, monitor *model.Monitor, eventType string) error {
	var cfg model.WebhookConfig
	if err := json.Unmarshal([]byte(ch.Config), &cfg); err != nil {
		return fmt.Errorf("parsing webhook config: %w", err)
	}

	eventName := "incident.confirmed"
	if eventType == "recovery" {
		eventName = "incident.resolved"
	}

	now := time.Now()
	payload := model.WebhookPayload{
		Event:     eventName,
		Timestamp: now.Format(time.RFC3339),
		Incident:  buildIncidentDetail(incident),
		Monitor:   buildMonitorSummary(monitor),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, cfg.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "PingMesh/1.0")

	if cfg.Secret != "" {
		mac := hmac.New(sha256.New, []byte(cfg.Secret))
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-PingMesh-Signature", "sha256="+sig)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func (d *Dispatcher) sendEmail(ch *model.AlertChannel, incident *model.Incident, monitor *model.Monitor, eventType string) error {
	var cfg model.EmailConfig
	if err := json.Unmarshal([]byte(ch.Config), &cfg); err != nil {
		return fmt.Errorf("parsing email config: %w", err)
	}

	status := "DOWN"
	if eventType == "recovery" {
		status = "RECOVERED"
	}
	label := "ALERT"
	if eventType == "recovery" {
		label = "RECOVERY"
	}

	subject := fmt.Sprintf("[PingMesh] %s: %s (%s) is %s", label, monitor.Name, monitor.Target, status)

	startedAt := time.UnixMilli(incident.StartedAt).Format(time.RFC3339)
	body := fmt.Sprintf("PingMesh Alert\n\nEvent: %s\nMonitor: %s\nType: %s\nTarget: %s\nGroup: %s\n\nIncident ID: %s\nStarted At: %s\nConfirming Nodes: %d\n",
		eventType, monitor.Name, monitor.CheckType, monitor.Target, monitor.GroupName,
		incident.ID, startedAt, len(incident.ConfirmingNodes))

	if incident.ConfirmedAt > 0 {
		body += fmt.Sprintf("Confirmed At: %s\n", time.UnixMilli(incident.ConfirmedAt).Format(time.RFC3339))
	}
	if incident.ResolvedAt > 0 {
		body += fmt.Sprintf("Resolved At: %s\n", time.UnixMilli(incident.ResolvedAt).Format(time.RFC3339))
		dur := time.Duration(incident.ResolvedAt-incident.StartedAt) * time.Millisecond
		body += fmt.Sprintf("Duration: %s\n", dur.Truncate(time.Second))
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		cfg.From, cfg.To, subject, body)

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)

	if cfg.TLS {
		return sendMailTLS(addr, auth, cfg.From, cfg.To, []byte(msg), cfg.SMTPHost)
	}
	return smtp.SendMail(addr, auth, cfg.From, []string{cfg.To}, []byte(msg))
}

func sendMailTLS(addr string, auth smtp.Auth, from, to string, msg []byte, host string) error {
	tlsConfig := &tls.Config{ServerName: host}
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS dial: %w", err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("SMTP client: %w", err)
	}
	defer client.Close()

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP auth: %w", err)
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("SMTP RCPT: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("SMTP write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("SMTP close data: %w", err)
	}

	return client.Quit()
}

func buildIncidentDetail(inc *model.Incident) model.IncidentDetail {
	d := model.IncidentDetail{
		ID:              inc.ID,
		Status:          string(inc.Status),
		StartedAt:       time.UnixMilli(inc.StartedAt).Format(time.RFC3339),
		ConfirmingNodes: inc.ConfirmingNodes,
	}
	if d.ConfirmingNodes == nil {
		d.ConfirmingNodes = []string{}
	}
	if inc.ConfirmedAt > 0 {
		d.ConfirmedAt = time.UnixMilli(inc.ConfirmedAt).Format(time.RFC3339)
	}
	if inc.ResolvedAt > 0 {
		d.ResolvedAt = time.UnixMilli(inc.ResolvedAt).Format(time.RFC3339)
		d.DurationSec = (inc.ResolvedAt - inc.StartedAt) / 1000
	}
	return d
}

func buildMonitorSummary(m *model.Monitor) model.MonitorSummary {
	return model.MonitorSummary{
		ID:        m.ID,
		Name:      m.Name,
		CheckType: string(m.CheckType),
		Target:    m.Target,
		Group:     m.GroupName,
	}
}
