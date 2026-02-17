package model

import (
	"encoding/json"
	"time"
)

// Node represents a member of the PingMesh cluster.
type Node struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Location  string `json:"location"`
	Address   string `json:"address"` // host:port for mTLS API
	Role      string `json:"role"`    // "coordinator" or "node"
	Status    string `json:"status"`  // "online", "offline", "suspect"
	LastSeen  int64  `json:"last_seen"`
	CreatedAt int64  `json:"created_at"`
}

const (
	RoleCoordinator = "coordinator"
	RoleNode        = "node"

	NodeOnline  = "online"
	NodeOffline = "offline"
	NodeSuspect = "suspect"
)

// CheckType represents the type of monitoring check.
type CheckType string

const (
	CheckICMP        CheckType = "icmp"
	CheckTCP         CheckType = "tcp"
	CheckHTTP        CheckType = "http"
	CheckHTTPS       CheckType = "https"
	CheckDNS         CheckType = "dns"
	CheckHTTPKeyword CheckType = "http_keyword"
)

// Monitor defines a monitoring check configuration.
type Monitor struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	GroupName         string    `json:"group_name"`
	CheckType         CheckType `json:"check_type"`
	Target            string    `json:"target"`
	Port              int       `json:"port,omitempty"`
	IntervalMS        int64     `json:"interval_ms"`
	TimeoutMS         int64     `json:"timeout_ms"`
	Retries           int       `json:"retries"`
	ExpectedStatus    int       `json:"expected_status,omitempty"`
	ExpectedKeyword   string    `json:"expected_keyword,omitempty"`
	DNSRecordType     string    `json:"dns_record_type,omitempty"`
	DNSExpected       string    `json:"dns_expected,omitempty"`
	FailureThreshold  int       `json:"failure_threshold"`
	RecoveryThreshold int       `json:"recovery_threshold"`
	QuorumType        string    `json:"quorum_type"` // "majority" or "n_of_m"
	QuorumN           int       `json:"quorum_n"`
	CooldownMS        int64     `json:"cooldown_ms"`
	Enabled           bool      `json:"enabled"`
	CreatedAt         int64     `json:"created_at"`
	UpdatedAt         int64     `json:"updated_at"`
}

// CheckStatus represents the outcome of a check.
type CheckStatus string

const (
	StatusUp       CheckStatus = "up"
	StatusDown     CheckStatus = "down"
	StatusDegraded CheckStatus = "degraded"
)

// CheckResult stores the outcome of a single check execution.
type CheckResult struct {
	ID         int64           `json:"id"`
	MonitorID  string          `json:"monitor_id"`
	NodeID     string          `json:"node_id"`
	Status     CheckStatus     `json:"status"`
	LatencyMS  float64         `json:"latency_ms"`
	StatusCode int             `json:"status_code,omitempty"`
	Error      string          `json:"error,omitempty"`
	Details    json.RawMessage `json:"details,omitempty"`
	Timestamp  int64           `json:"timestamp"`
}

// IncidentStatus represents the lifecycle state of an incident.
type IncidentStatus string

const (
	IncidentSuspect   IncidentStatus = "suspect"
	IncidentConfirmed IncidentStatus = "confirmed"
	IncidentResolved  IncidentStatus = "resolved"
)

// Incident represents a detected outage.
type Incident struct {
	ID              string         `json:"id"`
	MonitorID       string         `json:"monitor_id"`
	Status          IncidentStatus `json:"status"`
	StartedAt       int64          `json:"started_at"`
	ConfirmedAt     int64          `json:"confirmed_at,omitempty"`
	ResolvedAt      int64          `json:"resolved_at,omitempty"`
	ConfirmingNodes []string       `json:"confirming_nodes,omitempty"`
	CreatedAt       int64          `json:"created_at"`
	UpdatedAt       int64          `json:"updated_at"`
}

// PeerCheckRequest is sent to ask a peer node to run a check.
type PeerCheckRequest struct {
	RequestID   string `json:"request_id"`
	MonitorID   string `json:"monitor_id"`
	RequestedBy string `json:"requested_by"`
	Timestamp   string `json:"timestamp"`
}

// PeerCheckResponse is the result of a peer check request.
type PeerCheckResponse struct {
	RequestID string          `json:"request_id"`
	NodeID    string          `json:"node_id"`
	Result    PeerCheckResult `json:"result"`
}

// PeerCheckResult holds the actual check result data within a peer response.
type PeerCheckResult struct {
	Status     CheckStatus     `json:"status"`
	LatencyMS  float64         `json:"latency_ms"`
	Error      string          `json:"error,omitempty"`
	StatusCode *int            `json:"status_code"`
	Details    json.RawMessage `json:"details,omitempty"`
	Timestamp  string          `json:"timestamp"`
}

// Heartbeat is sent periodically between nodes.
type Heartbeat struct {
	NodeID          string `json:"node_id"`
	Timestamp       string `json:"timestamp"`
	ActiveMonitors  int    `json:"active_monitors"`
	ChecksPerMinute int    `json:"checks_per_minute"`
}

// ConfigSync is sent from coordinator to nodes for configuration distribution.
type ConfigSync struct {
	Version  int64     `json:"version"`
	Monitors []Monitor `json:"monitors"`
	Nodes    []Node    `json:"nodes"`
}

// JoinToken contains the data encoded in a join token.
type JoinToken struct {
	CoordinatorAddr string    `json:"addr"`
	Secret          []byte    `json:"secret"`
	ExpiresAt       time.Time `json:"expires_at"`
}

// JoinRequest is sent by a node to the coordinator to join the cluster.
type JoinRequest struct {
	Secret     []byte `json:"secret"`
	Name       string `json:"name"`
	ListenAddr string `json:"listen_addr"`
	CLIAddr    string `json:"cli_addr"`
}

// JoinResponse is returned by the coordinator after a successful join.
type JoinResponse struct {
	NodeID        string `json:"node_id"`
	CACert        string `json:"ca_cert"`
	NodeCert      string `json:"node_cert"`
	NodeKey       string `json:"node_key"`
	CoordinatorID string `json:"coordinator_id"`
}

// ClusterStatus provides an overview of the cluster state.
type ClusterStatus struct {
	NodeID          string     `json:"node_id"`
	Role            string     `json:"role"`
	Nodes           []Node     `json:"nodes"`
	MonitorCount    int        `json:"monitor_count"`
	ActiveIncidents []Incident `json:"active_incidents"`
}

// HealthInfo provides local node health information.
type HealthInfo struct {
	NodeID        string  `json:"node_id"`
	Name          string  `json:"name"`
	Role          string  `json:"role"`
	Uptime        string  `json:"uptime"`
	GoVersion     string  `json:"go_version"`
	NumGoroutines int     `json:"num_goroutines"`
	MemoryMB      float64 `json:"memory_mb"`
	DBSizeMB      float64 `json:"db_size_mb"`
	Coordinator   string  `json:"coordinator,omitempty"`
}
