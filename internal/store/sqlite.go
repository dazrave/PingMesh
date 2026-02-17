package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pingmesh/pingmesh/internal/model"
	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens or creates a SQLite database at the given path.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)&_pragma=synchronous(normal)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(on)")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite single-writer
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return s, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// --- Node operations ---

func (s *SQLiteStore) CreateNode(node *model.Node) error {
	_, err := s.db.Exec(
		`INSERT INTO nodes (id, name, location, address, role, status, last_seen, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		node.ID, node.Name, node.Location, node.Address, node.Role, node.Status, node.LastSeen, node.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) GetNode(id string) (*model.Node, error) {
	row := s.db.QueryRow(`SELECT id, name, location, address, role, status, last_seen, created_at FROM nodes WHERE id = ?`, id)
	var n model.Node
	err := row.Scan(&n.ID, &n.Name, &n.Location, &n.Address, &n.Role, &n.Status, &n.LastSeen, &n.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (s *SQLiteStore) ListNodes() ([]model.Node, error) {
	rows, err := s.db.Query(`SELECT id, name, location, address, role, status, last_seen, created_at FROM nodes ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []model.Node
	for rows.Next() {
		var n model.Node
		if err := rows.Scan(&n.ID, &n.Name, &n.Location, &n.Address, &n.Role, &n.Status, &n.LastSeen, &n.CreatedAt); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func (s *SQLiteStore) UpdateNode(node *model.Node) error {
	_, err := s.db.Exec(
		`UPDATE nodes SET name = ?, location = ?, address = ?, role = ?, status = ?, last_seen = ? WHERE id = ?`,
		node.Name, node.Location, node.Address, node.Role, node.Status, node.LastSeen, node.ID,
	)
	return err
}

func (s *SQLiteStore) DeleteNode(id string) error {
	_, err := s.db.Exec(`DELETE FROM nodes WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) UpdateNodeStatus(id string, status string, lastSeen int64) error {
	_, err := s.db.Exec(`UPDATE nodes SET status = ?, last_seen = ? WHERE id = ?`, status, lastSeen, id)
	return err
}

// --- Monitor operations ---

func (s *SQLiteStore) CreateMonitor(monitor *model.Monitor) error {
	_, err := s.db.Exec(
		`INSERT INTO monitors (id, name, group_name, check_type, target, port, interval_ms, timeout_ms,
		 retries, expected_status, expected_keyword, dns_record_type, dns_expected,
		 failure_threshold, recovery_threshold, quorum_type, quorum_n, cooldown_ms, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		monitor.ID, monitor.Name, monitor.GroupName, string(monitor.CheckType), monitor.Target,
		nullInt(monitor.Port), monitor.IntervalMS, monitor.TimeoutMS, monitor.Retries,
		nullInt(monitor.ExpectedStatus), nullString(monitor.ExpectedKeyword),
		nullString(monitor.DNSRecordType), nullString(monitor.DNSExpected),
		monitor.FailureThreshold, monitor.RecoveryThreshold,
		monitor.QuorumType, monitor.QuorumN, monitor.CooldownMS,
		boolToInt(monitor.Enabled), monitor.CreatedAt, monitor.UpdatedAt,
	)
	return err
}

func (s *SQLiteStore) GetMonitor(id string) (*model.Monitor, error) {
	row := s.db.QueryRow(
		`SELECT id, name, group_name, check_type, target, port, interval_ms, timeout_ms,
		 retries, expected_status, expected_keyword, dns_record_type, dns_expected,
		 failure_threshold, recovery_threshold, quorum_type, quorum_n, cooldown_ms, enabled, created_at, updated_at
		 FROM monitors WHERE id = ?`, id)

	return scanMonitor(row)
}

func (s *SQLiteStore) ListMonitors(groupName string) ([]model.Monitor, error) {
	var rows *sql.Rows
	var err error
	if groupName != "" {
		rows, err = s.db.Query(
			`SELECT id, name, group_name, check_type, target, port, interval_ms, timeout_ms,
			 retries, expected_status, expected_keyword, dns_record_type, dns_expected,
			 failure_threshold, recovery_threshold, quorum_type, quorum_n, cooldown_ms, enabled, created_at, updated_at
			 FROM monitors WHERE group_name = ? ORDER BY name`, groupName)
	} else {
		rows, err = s.db.Query(
			`SELECT id, name, group_name, check_type, target, port, interval_ms, timeout_ms,
			 retries, expected_status, expected_keyword, dns_record_type, dns_expected,
			 failure_threshold, recovery_threshold, quorum_type, quorum_n, cooldown_ms, enabled, created_at, updated_at
			 FROM monitors ORDER BY name`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var monitors []model.Monitor
	for rows.Next() {
		m, err := scanMonitorRow(rows)
		if err != nil {
			return nil, err
		}
		monitors = append(monitors, *m)
	}
	return monitors, rows.Err()
}

func (s *SQLiteStore) UpdateMonitor(monitor *model.Monitor) error {
	_, err := s.db.Exec(
		`UPDATE monitors SET name = ?, group_name = ?, check_type = ?, target = ?, port = ?,
		 interval_ms = ?, timeout_ms = ?, retries = ?, expected_status = ?, expected_keyword = ?,
		 dns_record_type = ?, dns_expected = ?, failure_threshold = ?, recovery_threshold = ?,
		 quorum_type = ?, quorum_n = ?, cooldown_ms = ?, enabled = ?, updated_at = ?
		 WHERE id = ?`,
		monitor.Name, monitor.GroupName, string(monitor.CheckType), monitor.Target,
		nullInt(monitor.Port), monitor.IntervalMS, monitor.TimeoutMS, monitor.Retries,
		nullInt(monitor.ExpectedStatus), nullString(monitor.ExpectedKeyword),
		nullString(monitor.DNSRecordType), nullString(monitor.DNSExpected),
		monitor.FailureThreshold, monitor.RecoveryThreshold,
		monitor.QuorumType, monitor.QuorumN, monitor.CooldownMS,
		boolToInt(monitor.Enabled), monitor.UpdatedAt, monitor.ID,
	)
	return err
}

func (s *SQLiteStore) DeleteMonitor(id string) error {
	_, err := s.db.Exec(`DELETE FROM monitors WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) ListEnabledMonitors() ([]model.Monitor, error) {
	rows, err := s.db.Query(
		`SELECT id, name, group_name, check_type, target, port, interval_ms, timeout_ms,
		 retries, expected_status, expected_keyword, dns_record_type, dns_expected,
		 failure_threshold, recovery_threshold, quorum_type, quorum_n, cooldown_ms, enabled, created_at, updated_at
		 FROM monitors WHERE enabled = 1 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var monitors []model.Monitor
	for rows.Next() {
		m, err := scanMonitorRow(rows)
		if err != nil {
			return nil, err
		}
		monitors = append(monitors, *m)
	}
	return monitors, rows.Err()
}

// --- Check result operations ---

func (s *SQLiteStore) InsertCheckResult(result *model.CheckResult) error {
	details := ""
	if result.Details != nil {
		details = string(result.Details)
	}
	_, err := s.db.Exec(
		`INSERT INTO check_results (monitor_id, node_id, status, latency_ms, status_code, error, details, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		result.MonitorID, result.NodeID, string(result.Status), result.LatencyMS,
		nullInt(result.StatusCode), nullString(result.Error), nullString(details), result.Timestamp,
	)
	return err
}

func (s *SQLiteStore) GetLatestResult(monitorID, nodeID string) (*model.CheckResult, error) {
	row := s.db.QueryRow(
		`SELECT id, monitor_id, node_id, status, latency_ms, status_code, error, details, timestamp
		 FROM check_results WHERE monitor_id = ? AND node_id = ? ORDER BY timestamp DESC LIMIT 1`,
		monitorID, nodeID)

	var r model.CheckResult
	var statusCode sql.NullInt64
	var errStr sql.NullString
	var details sql.NullString
	err := row.Scan(&r.ID, &r.MonitorID, &r.NodeID, &r.Status, &r.LatencyMS, &statusCode, &errStr, &details, &r.Timestamp)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if statusCode.Valid {
		r.StatusCode = int(statusCode.Int64)
	}
	if errStr.Valid {
		r.Error = errStr.String
	}
	if details.Valid && details.String != "" {
		r.Details = json.RawMessage(details.String)
	}
	return &r, nil
}

func (s *SQLiteStore) CountConsecutiveFailures(monitorID, nodeID string) (int, error) {
	rows, err := s.db.Query(
		`SELECT status FROM check_results
		 WHERE monitor_id = ? AND node_id = ?
		 ORDER BY timestamp DESC LIMIT 100`,
		monitorID, nodeID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			return 0, err
		}
		if status != string(model.StatusUp) {
			count++
		} else {
			break
		}
	}
	return count, rows.Err()
}

func (s *SQLiteStore) CountConsecutiveSuccesses(monitorID, nodeID string) (int, error) {
	rows, err := s.db.Query(
		`SELECT status FROM check_results
		 WHERE monitor_id = ? AND node_id = ?
		 ORDER BY timestamp DESC LIMIT 100`,
		monitorID, nodeID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			return 0, err
		}
		if status == string(model.StatusUp) {
			count++
		} else {
			break
		}
	}
	return count, rows.Err()
}

func (s *SQLiteStore) ListCheckResults(monitorID, nodeID string, since int64, limit int) ([]model.CheckResult, error) {
	query := `SELECT id, monitor_id, node_id, status, latency_ms, status_code, error, details, timestamp
		 FROM check_results WHERE 1=1`
	args := []any{}

	if monitorID != "" {
		query += ` AND monitor_id = ?`
		args = append(args, monitorID)
	}
	if nodeID != "" {
		query += ` AND node_id = ?`
		args = append(args, nodeID)
	}
	if since > 0 {
		query += ` AND timestamp >= ?`
		args = append(args, since)
	}
	query += ` ORDER BY timestamp DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.CheckResult
	for rows.Next() {
		var r model.CheckResult
		var statusCode sql.NullInt64
		var errStr sql.NullString
		var details sql.NullString
		if err := rows.Scan(&r.ID, &r.MonitorID, &r.NodeID, &r.Status, &r.LatencyMS, &statusCode, &errStr, &details, &r.Timestamp); err != nil {
			return nil, err
		}
		if statusCode.Valid {
			r.StatusCode = int(statusCode.Int64)
		}
		if errStr.Valid {
			r.Error = errStr.String
		}
		if details.Valid && details.String != "" {
			r.Details = json.RawMessage(details.String)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// --- Incident operations ---

func (s *SQLiteStore) CreateIncident(incident *model.Incident) error {
	nodesJSON, _ := json.Marshal(incident.ConfirmingNodes)
	_, err := s.db.Exec(
		`INSERT INTO incidents (id, monitor_id, status, started_at, confirmed_at, resolved_at, confirming_nodes, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		incident.ID, incident.MonitorID, string(incident.Status), incident.StartedAt,
		nullInt64(incident.ConfirmedAt), nullInt64(incident.ResolvedAt),
		string(nodesJSON), incident.CreatedAt, incident.UpdatedAt,
	)
	return err
}

func (s *SQLiteStore) GetIncident(id string) (*model.Incident, error) {
	row := s.db.QueryRow(
		`SELECT id, monitor_id, status, started_at, confirmed_at, resolved_at, confirming_nodes, created_at, updated_at
		 FROM incidents WHERE id = ?`, id)
	return scanIncident(row)
}

func (s *SQLiteStore) GetActiveIncident(monitorID string) (*model.Incident, error) {
	row := s.db.QueryRow(
		`SELECT id, monitor_id, status, started_at, confirmed_at, resolved_at, confirming_nodes, created_at, updated_at
		 FROM incidents WHERE monitor_id = ? AND status != 'resolved' ORDER BY created_at DESC LIMIT 1`, monitorID)
	return scanIncident(row)
}

func (s *SQLiteStore) UpdateIncident(incident *model.Incident) error {
	nodesJSON, _ := json.Marshal(incident.ConfirmingNodes)
	_, err := s.db.Exec(
		`UPDATE incidents SET status = ?, confirmed_at = ?, resolved_at = ?, confirming_nodes = ?, updated_at = ?
		 WHERE id = ?`,
		string(incident.Status), nullInt64(incident.ConfirmedAt), nullInt64(incident.ResolvedAt),
		string(nodesJSON), incident.UpdatedAt, incident.ID,
	)
	return err
}

func (s *SQLiteStore) ListIncidents(activeOnly bool) ([]model.Incident, error) {
	query := `SELECT id, monitor_id, status, started_at, confirmed_at, resolved_at, confirming_nodes, created_at, updated_at FROM incidents`
	if activeOnly {
		query += ` WHERE status != 'resolved'`
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var incidents []model.Incident
	for rows.Next() {
		inc, err := scanIncidentRow(rows)
		if err != nil {
			return nil, err
		}
		incidents = append(incidents, *inc)
	}
	return incidents, rows.Err()
}

// --- Join token operations ---

func (s *SQLiteStore) StoreJoinToken(tokenHash string, expiresAt int64) error {
	_, err := s.db.Exec(
		`INSERT INTO join_tokens (token_hash, expires_at, used, created_at) VALUES (?, ?, 0, ?)`,
		tokenHash, expiresAt, time.Now().UnixMilli(),
	)
	return err
}

func (s *SQLiteStore) ValidateAndConsumeToken(tokenHash string) (bool, error) {
	result, err := s.db.Exec(
		`UPDATE join_tokens SET used = 1 WHERE token_hash = ? AND used = 0 AND expires_at > ?`,
		tokenHash, time.Now().UnixMilli(),
	)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// --- Alert channel operations ---

func (s *SQLiteStore) CreateAlertChannel(ch *model.AlertChannel) error {
	_, err := s.db.Exec(
		`INSERT INTO alert_channels (id, name, type, enabled, config, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ch.ID, ch.Name, ch.Type, boolToInt(ch.Enabled), ch.Config, ch.CreatedAt, ch.UpdatedAt,
	)
	return err
}

func (s *SQLiteStore) GetAlertChannel(id string) (*model.AlertChannel, error) {
	row := s.db.QueryRow(
		`SELECT id, name, type, enabled, config, created_at, updated_at FROM alert_channels WHERE id = ?`, id)
	return scanAlertChannel(row)
}

func (s *SQLiteStore) ListAlertChannels() ([]model.AlertChannel, error) {
	rows, err := s.db.Query(
		`SELECT id, name, type, enabled, config, created_at, updated_at FROM alert_channels ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []model.AlertChannel
	for rows.Next() {
		ch, err := scanAlertChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, *ch)
	}
	return channels, rows.Err()
}

func (s *SQLiteStore) ListEnabledAlertChannels() ([]model.AlertChannel, error) {
	rows, err := s.db.Query(
		`SELECT id, name, type, enabled, config, created_at, updated_at FROM alert_channels WHERE enabled = 1 ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []model.AlertChannel
	for rows.Next() {
		ch, err := scanAlertChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, *ch)
	}
	return channels, rows.Err()
}

func (s *SQLiteStore) UpdateAlertChannel(ch *model.AlertChannel) error {
	_, err := s.db.Exec(
		`UPDATE alert_channels SET name = ?, type = ?, enabled = ?, config = ?, updated_at = ? WHERE id = ?`,
		ch.Name, ch.Type, boolToInt(ch.Enabled), ch.Config, ch.UpdatedAt, ch.ID,
	)
	return err
}

func (s *SQLiteStore) DeleteAlertChannel(id string) error {
	_, err := s.db.Exec(`DELETE FROM alert_channels WHERE id = ?`, id)
	return err
}

// --- Alert history operations ---

func (s *SQLiteStore) InsertAlertRecord(rec *model.AlertRecord) error {
	_, err := s.db.Exec(
		`INSERT INTO alert_history (channel_id, incident_id, monitor_id, event_type, status, error, sent_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rec.ChannelID, rec.IncidentID, rec.MonitorID, rec.EventType, rec.Status,
		nullString(rec.Error), rec.SentAt,
	)
	return err
}

func (s *SQLiteStore) ListAlertHistory(channelID string, limit int) ([]model.AlertRecord, error) {
	query := `SELECT id, channel_id, incident_id, monitor_id, event_type, status, error, sent_at FROM alert_history`
	var args []any

	if channelID != "" {
		query += ` WHERE channel_id = ?`
		args = append(args, channelID)
	}
	query += ` ORDER BY sent_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []model.AlertRecord
	for rows.Next() {
		var rec model.AlertRecord
		var errStr sql.NullString
		if err := rows.Scan(&rec.ID, &rec.ChannelID, &rec.IncidentID, &rec.MonitorID,
			&rec.EventType, &rec.Status, &errStr, &rec.SentAt); err != nil {
			return nil, err
		}
		if errStr.Valid {
			rec.Error = errStr.String
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

func scanAlertChannel(row scannable) (*model.AlertChannel, error) {
	var ch model.AlertChannel
	var enabled int
	err := row.Scan(&ch.ID, &ch.Name, &ch.Type, &enabled, &ch.Config, &ch.CreatedAt, &ch.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ch.Enabled = enabled == 1
	return &ch, nil
}

// --- Helper functions ---

type scannable interface {
	Scan(dest ...any) error
}

func scanMonitor(row scannable) (*model.Monitor, error) {
	var m model.Monitor
	var port sql.NullInt64
	var expectedStatus sql.NullInt64
	var expectedKeyword sql.NullString
	var dnsRecordType sql.NullString
	var dnsExpected sql.NullString
	var enabled int

	err := row.Scan(
		&m.ID, &m.Name, &m.GroupName, &m.CheckType, &m.Target, &port,
		&m.IntervalMS, &m.TimeoutMS, &m.Retries, &expectedStatus, &expectedKeyword,
		&dnsRecordType, &dnsExpected, &m.FailureThreshold, &m.RecoveryThreshold,
		&m.QuorumType, &m.QuorumN, &m.CooldownMS, &enabled, &m.CreatedAt, &m.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if port.Valid {
		m.Port = int(port.Int64)
	}
	if expectedStatus.Valid {
		m.ExpectedStatus = int(expectedStatus.Int64)
	}
	if expectedKeyword.Valid {
		m.ExpectedKeyword = expectedKeyword.String
	}
	if dnsRecordType.Valid {
		m.DNSRecordType = dnsRecordType.String
	}
	if dnsExpected.Valid {
		m.DNSExpected = dnsExpected.String
	}
	m.Enabled = enabled == 1

	return &m, nil
}

func scanMonitorRow(rows *sql.Rows) (*model.Monitor, error) {
	return scanMonitor(rows)
}

func scanIncident(row scannable) (*model.Incident, error) {
	var inc model.Incident
	var confirmedAt sql.NullInt64
	var resolvedAt sql.NullInt64
	var nodesJSON string

	err := row.Scan(&inc.ID, &inc.MonitorID, &inc.Status, &inc.StartedAt,
		&confirmedAt, &resolvedAt, &nodesJSON, &inc.CreatedAt, &inc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if confirmedAt.Valid {
		inc.ConfirmedAt = confirmedAt.Int64
	}
	if resolvedAt.Valid {
		inc.ResolvedAt = resolvedAt.Int64
	}
	if nodesJSON != "" {
		json.Unmarshal([]byte(nodesJSON), &inc.ConfirmingNodes)
	}

	return &inc, nil
}

func scanIncidentRow(rows *sql.Rows) (*model.Incident, error) {
	return scanIncident(rows)
}

func nullInt(v int) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(v), Valid: true}
}

func nullInt64(v int64) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: v, Valid: true}
}

func nullString(v string) sql.NullString {
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
