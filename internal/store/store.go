package store

import (
	"github.com/pingmesh/pingmesh/internal/model"
)

// Store defines the persistence interface for PingMesh.
type Store interface {
	// Node operations
	CreateNode(node *model.Node) error
	GetNode(id string) (*model.Node, error)
	ListNodes() ([]model.Node, error)
	UpdateNode(node *model.Node) error
	DeleteNode(id string) error
	UpdateNodeStatus(id string, status string, lastSeen int64) error

	// Monitor operations
	CreateMonitor(monitor *model.Monitor) error
	GetMonitor(id string) (*model.Monitor, error)
	ListMonitors(groupName string) ([]model.Monitor, error)
	UpdateMonitor(monitor *model.Monitor) error
	DeleteMonitor(id string) error
	ListEnabledMonitors() ([]model.Monitor, error)

	// Check result operations
	InsertCheckResult(result *model.CheckResult) error
	GetLatestResult(monitorID, nodeID string) (*model.CheckResult, error)
	CountConsecutiveFailures(monitorID, nodeID string) (int, error)
	CountConsecutiveSuccesses(monitorID, nodeID string) (int, error)
	ListCheckResults(monitorID, nodeID string, since int64, limit int) ([]model.CheckResult, error)

	// Incident operations
	CreateIncident(incident *model.Incident) error
	GetIncident(id string) (*model.Incident, error)
	GetActiveIncident(monitorID string) (*model.Incident, error)
	UpdateIncident(incident *model.Incident) error
	ListIncidents(activeOnly bool) ([]model.Incident, error)

	// Join token operations
	StoreJoinToken(tokenHash string, expiresAt int64) error
	ValidateAndConsumeToken(tokenHash string) (bool, error)

	// Lifecycle
	Close() error
}
