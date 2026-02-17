package cluster

import (
	"log"
	"time"

	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/pingmesh/pingmesh/internal/model"
	"github.com/pingmesh/pingmesh/internal/store"
)

// Manager handles cluster membership and communication.
type Manager struct {
	config *config.Config
	store  store.Store
}

// NewManager creates a new cluster manager.
func NewManager(cfg *config.Config, st store.Store) *Manager {
	return &Manager{
		config: cfg,
		store:  st,
	}
}

// GetOnlineNodes returns all nodes with status "online".
func (m *Manager) GetOnlineNodes() ([]model.Node, error) {
	nodes, err := m.store.ListNodes()
	if err != nil {
		return nil, err
	}

	var online []model.Node
	for _, n := range nodes {
		if n.Status == model.NodeOnline {
			online = append(online, n)
		}
	}
	return online, nil
}

// UpdateHeartbeat updates a node's last-seen timestamp.
func (m *Manager) UpdateHeartbeat(nodeID string) error {
	return m.store.UpdateNodeStatus(nodeID, model.NodeOnline, time.Now().UnixMilli())
}

// DetectOfflineNodes marks nodes as offline if they haven't been seen recently.
// TODO (M3): Implement periodic offline detection
func (m *Manager) DetectOfflineNodes(timeoutMS int64) error {
	nodes, err := m.store.ListNodes()
	if err != nil {
		return err
	}

	cutoff := time.Now().UnixMilli() - timeoutMS
	for _, n := range nodes {
		if n.LastSeen < cutoff && n.Status == model.NodeOnline {
			log.Printf("[cluster] marking node %s (%s) as offline", n.ID, n.Name)
			if err := m.store.UpdateNodeStatus(n.ID, model.NodeOffline, n.LastSeen); err != nil {
				log.Printf("[cluster] error updating node status: %v", err)
			}
		}
	}
	return nil
}
