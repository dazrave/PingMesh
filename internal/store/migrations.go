package store

const schemaVersion = 1

const migrationSQL = `
CREATE TABLE IF NOT EXISTS nodes (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    location    TEXT NOT NULL DEFAULT '',
    address     TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'node',
    status      TEXT NOT NULL DEFAULT 'online',
    last_seen   INTEGER NOT NULL,
    created_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS monitors (
    id                  TEXT PRIMARY KEY,
    name                TEXT NOT NULL,
    group_name          TEXT NOT NULL DEFAULT '',
    check_type          TEXT NOT NULL,
    target              TEXT NOT NULL,
    port                INTEGER,
    interval_ms         INTEGER NOT NULL DEFAULT 60000,
    timeout_ms          INTEGER NOT NULL DEFAULT 5000,
    retries             INTEGER NOT NULL DEFAULT 1,
    expected_status     INTEGER,
    expected_keyword    TEXT,
    dns_record_type     TEXT,
    dns_expected        TEXT,
    failure_threshold   INTEGER NOT NULL DEFAULT 3,
    recovery_threshold  INTEGER NOT NULL DEFAULT 2,
    quorum_type         TEXT NOT NULL DEFAULT 'majority',
    quorum_n            INTEGER NOT NULL DEFAULT 0,
    cooldown_ms         INTEGER NOT NULL DEFAULT 300000,
    enabled             INTEGER NOT NULL DEFAULT 1,
    created_at          INTEGER NOT NULL,
    updated_at          INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS check_results (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    monitor_id  TEXT NOT NULL REFERENCES monitors(id),
    node_id     TEXT NOT NULL REFERENCES nodes(id),
    status      TEXT NOT NULL,
    latency_ms  REAL,
    status_code INTEGER,
    error       TEXT,
    details     TEXT,
    timestamp   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_results_monitor_node ON check_results(monitor_id, node_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_results_timestamp ON check_results(timestamp);

CREATE TABLE IF NOT EXISTS incidents (
    id              TEXT PRIMARY KEY,
    monitor_id      TEXT NOT NULL REFERENCES monitors(id),
    status          TEXT NOT NULL,
    started_at      INTEGER NOT NULL,
    confirmed_at    INTEGER,
    resolved_at     INTEGER,
    confirming_nodes TEXT,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_incidents_monitor ON incidents(monitor_id, status);

CREATE TABLE IF NOT EXISTS join_tokens (
    token_hash  TEXT PRIMARY KEY,
    expires_at  INTEGER NOT NULL,
    used        INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER NOT NULL
);
`

func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(migrationSQL)
	if err != nil {
		return err
	}

	// Set schema version
	_, err = s.db.Exec(`INSERT OR REPLACE INTO schema_version (rowid, version) VALUES (1, ?)`, schemaVersion)
	return err
}
