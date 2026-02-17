package consensus

import (
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/pingmesh/pingmesh/internal/model"
	"github.com/pingmesh/pingmesh/internal/store"
)

// IncidentManager handles incident lifecycle state transitions.
type IncidentManager struct {
	store store.Store
}

// NewIncidentManager creates a new incident manager.
func NewIncidentManager(st store.Store) *IncidentManager {
	return &IncidentManager{store: st}
}

// GetOrCreateIncident returns the active incident for a monitor, or creates a new one.
func (m *IncidentManager) GetOrCreateIncident(monitorID string) (*model.Incident, error) {
	incident, err := m.store.GetActiveIncident(monitorID)
	if err != nil {
		return nil, err
	}
	if incident != nil {
		return incident, nil
	}

	now := time.Now().UnixMilli()
	incident = &model.Incident{
		ID:        uuid.New().String(),
		MonitorID: monitorID,
		Status:    model.IncidentSuspect,
		StartedAt: now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := m.store.CreateIncident(incident); err != nil {
		return nil, err
	}

	log.Printf("[incident] created suspect incident %s for monitor %s", incident.ID, monitorID)
	return incident, nil
}

// ConfirmIncident transitions an incident to confirmed status.
func (m *IncidentManager) ConfirmIncident(incident *model.Incident, confirmingNodes []string) error {
	now := time.Now().UnixMilli()
	incident.Status = model.IncidentConfirmed
	incident.ConfirmedAt = now
	incident.ConfirmingNodes = confirmingNodes
	incident.UpdatedAt = now

	log.Printf("[incident] confirmed incident %s for monitor %s by nodes %v",
		incident.ID, incident.MonitorID, confirmingNodes)
	return m.store.UpdateIncident(incident)
}

// ResolveIncident transitions an incident to resolved status.
func (m *IncidentManager) ResolveIncident(incident *model.Incident) error {
	now := time.Now().UnixMilli()
	incident.Status = model.IncidentResolved
	incident.ResolvedAt = now
	incident.UpdatedAt = now

	log.Printf("[incident] resolved incident %s for monitor %s", incident.ID, incident.MonitorID)
	return m.store.UpdateIncident(incident)
}
