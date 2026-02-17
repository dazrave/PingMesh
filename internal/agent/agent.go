package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/pingmesh/pingmesh/internal/alert"
	"github.com/pingmesh/pingmesh/internal/checker"
	"github.com/pingmesh/pingmesh/internal/cluster"
	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/pingmesh/pingmesh/internal/consensus"
	"github.com/pingmesh/pingmesh/internal/model"
	"github.com/pingmesh/pingmesh/internal/store"
)

// Agent is the main runtime that coordinates check scheduling, cluster communication, and API serving.
type Agent struct {
	config      *config.Config
	store       store.Store
	scheduler   *Scheduler
	peerClient  *cluster.PeerClient
	clusterMgr  *cluster.Manager
	incidentMgr *consensus.IncidentManager
	alerter     *alert.Dispatcher
	startTime   time.Time

	mu             sync.RWMutex
	lastHeartbeat  time.Time
	lastConfigSync time.Time
}

// New creates a new Agent instance.
func New(cfg *config.Config, st store.Store) *Agent {
	a := &Agent{
		config:      cfg,
		store:       st,
		scheduler:   NewScheduler(st, cfg.NodeID),
		peerClient:  cluster.NewPeerClient(),
		clusterMgr:  cluster.NewManager(cfg, st),
		incidentMgr: consensus.NewIncidentManager(st),
		alerter:     alert.NewDispatcher(),
		startTime:   time.Now(),
	}

	// Set up result callback: non-coordinators push results to coordinator
	if cfg.Role != model.RoleCoordinator && cfg.Coordinator != nil {
		coordAddr := cfg.Coordinator.Address
		a.scheduler.SetResultCallback(func(result *model.CheckResult) {
			if err := a.peerClient.PushResult(coordAddr, result); err != nil {
				log.Printf("[agent] failed to push result to coordinator: %v", err)
			}
		})
	}

	return a
}

// Run starts the agent and blocks until the context is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	log.Printf("[agent] starting node %s (%s) role=%s", a.config.NodeID, a.config.NodeName, a.config.Role)

	// Register all check types
	checker.RegisterAll()

	// Start the monitor sync loop
	go a.syncLoop(ctx)

	// Start cluster loops
	go a.heartbeatLoop(ctx)

	if a.config.Role == model.RoleCoordinator {
		go a.offlineDetectionLoop(ctx)
		go a.configSyncLoop(ctx)
		go a.consensusLoop(ctx)
	} else {
		// Non-coordinators pull config from coordinator
		go a.configPullLoop(ctx)
	}

	<-ctx.Done()
	log.Println("[agent] shutting down...")
	a.scheduler.Stop()
	return nil
}

// heartbeatLoop sends heartbeats every 30s.
func (a *Agent) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Send immediately on start
	a.sendHeartbeat()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.sendHeartbeat()
		}
	}
}

func (a *Agent) sendHeartbeat() {
	// Update own status locally
	if err := a.clusterMgr.UpdateHeartbeat(a.config.NodeID); err != nil {
		log.Printf("[agent] heartbeat self-update error: %v", err)
	}

	// Non-coordinators send heartbeat to coordinator
	if a.config.Role != model.RoleCoordinator && a.config.Coordinator != nil {
		hb := &model.Heartbeat{
			NodeID:         a.config.NodeID,
			Timestamp:      time.Now().Format(time.RFC3339),
			ActiveMonitors: a.scheduler.ActiveCount(),
		}
		if err := a.peerClient.SendHeartbeat(a.config.Coordinator.Address, hb); err != nil {
			log.Printf("[agent] failed to send heartbeat to coordinator: %v", err)
		}
	}

	a.mu.Lock()
	a.lastHeartbeat = time.Now()
	a.mu.Unlock()
}

// offlineDetectionLoop detects offline nodes every 30s (coordinator only).
func (a *Agent) offlineDetectionLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.clusterMgr.DetectOfflineNodes(90000); err != nil {
				log.Printf("[agent] offline detection error: %v", err)
			}
		}
	}
}

// configSyncLoop pushes config to all online nodes every 30s (coordinator only).
func (a *Agent) configSyncLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.pushConfigSync()
		}
	}
}

func (a *Agent) pushConfigSync() {
	monitors, err := a.store.ListMonitors("")
	if err != nil {
		log.Printf("[agent] config-sync: error loading monitors: %v", err)
		return
	}

	nodes, err := a.store.ListNodes()
	if err != nil {
		log.Printf("[agent] config-sync: error loading nodes: %v", err)
		return
	}

	sync := &model.ConfigSync{
		Version:  time.Now().UnixMilli(),
		Monitors: monitors,
		Nodes:    nodes,
	}

	for _, n := range nodes {
		// Skip self and offline nodes
		if n.ID == a.config.NodeID || n.Status != model.NodeOnline {
			continue
		}
		if err := a.peerClient.PushConfigSync(n.Address, sync); err != nil {
			log.Printf("[agent] config-sync to %s (%s) failed: %v", n.Name, n.Address, err)
		}
	}

	a.mu.Lock()
	a.lastConfigSync = time.Now()
	a.mu.Unlock()
}

// configPullLoop pulls config from the coordinator every 30s (non-coordinator only).
func (a *Agent) configPullLoop(ctx context.Context) {
	if a.config.Coordinator == nil {
		return
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Pull immediately on start
	a.pullConfigSync()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.pullConfigSync()
		}
	}
}

func (a *Agent) pullConfigSync() {
	coordAddr := a.config.Coordinator.Address
	sync, err := a.peerClient.PullConfigSync(coordAddr)
	if err != nil {
		log.Printf("[agent] config-pull from coordinator failed: %v", err)
		return
	}

	a.applyConfigSync(sync)

	a.mu.Lock()
	a.lastConfigSync = time.Now()
	a.mu.Unlock()
}

// applyConfigSync merges pulled config into the local store.
func (a *Agent) applyConfigSync(sync *model.ConfigSync) {
	for i := range sync.Monitors {
		m := &sync.Monitors[i]
		existing, err := a.store.GetMonitor(m.ID)
		if err != nil {
			log.Printf("[agent] config-pull: error checking monitor %s: %v", m.ID, err)
			continue
		}
		if existing != nil {
			if err := a.store.UpdateMonitor(m); err != nil {
				log.Printf("[agent] config-pull: error updating monitor %s: %v", m.ID, err)
			}
		} else {
			if err := a.store.CreateMonitor(m); err != nil {
				log.Printf("[agent] config-pull: error creating monitor %s: %v", m.ID, err)
			}
		}
	}

	for i := range sync.Nodes {
		n := &sync.Nodes[i]
		existing, err := a.store.GetNode(n.ID)
		if err != nil {
			log.Printf("[agent] config-pull: error checking node %s: %v", n.ID, err)
			continue
		}
		if existing != nil {
			if err := a.store.UpdateNode(n); err != nil {
				log.Printf("[agent] config-pull: error updating node %s: %v", n.ID, err)
			}
		} else {
			if err := a.store.CreateNode(n); err != nil {
				log.Printf("[agent] config-pull: error creating node %s: %v", n.ID, err)
			}
		}
	}

	log.Printf("[agent] config-pull applied: %d monitors, %d nodes", len(sync.Monitors), len(sync.Nodes))
}

// consensusLoop evaluates quorum for incidents every 15s (coordinator only).
func (a *Agent) consensusLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.evaluateConsensus()
		}
	}
}

func (a *Agent) evaluateConsensus() {
	monitors, err := a.store.ListEnabledMonitors()
	if err != nil {
		log.Printf("[consensus] error loading monitors: %v", err)
		return
	}

	onlineNodes, err := a.clusterMgr.GetOnlineNodes()
	if err != nil {
		log.Printf("[consensus] error getting online nodes: %v", err)
		return
	}

	totalNodes := len(onlineNodes)
	if totalNodes == 0 {
		return
	}

	for _, monitor := range monitors {
		a.evaluateMonitorConsensus(&monitor, onlineNodes, totalNodes)
	}
}

func (a *Agent) evaluateMonitorConsensus(monitor *model.Monitor, onlineNodes []model.Node, totalNodes int) {
	failCount := 0
	var failingNodeIDs []string

	for _, node := range onlineNodes {
		failures, err := a.store.CountConsecutiveFailures(monitor.ID, node.ID)
		if err != nil {
			log.Printf("[consensus] error counting failures for monitor=%s node=%s: %v", monitor.ID, node.ID, err)
			continue
		}
		if failures >= monitor.FailureThreshold {
			failCount++
			failingNodeIDs = append(failingNodeIDs, node.ID)
		}
	}

	quorumMet := consensus.EvaluateQuorum(monitor.QuorumType, monitor.QuorumN, failCount, totalNodes)

	if quorumMet {
		incident, err := a.incidentMgr.GetOrCreateIncident(monitor.ID)
		if err != nil {
			log.Printf("[consensus] error getting/creating incident for monitor %s: %v", monitor.ID, err)
			return
		}
		if incident.Status == model.IncidentSuspect {
			if err := a.incidentMgr.ConfirmIncident(incident, failingNodeIDs); err != nil {
				log.Printf("[consensus] error confirming incident %s: %v", incident.ID, err)
				return
			}
			a.alerter.SendAlert(incident, monitor)
		}
	} else {
		// Check if there's an active incident to resolve
		incident, err := a.store.GetActiveIncident(monitor.ID)
		if err != nil || incident == nil {
			return
		}

		// Count nodes with enough consecutive successes for recovery
		recoveryCount := 0
		for _, node := range onlineNodes {
			successes, err := a.store.CountConsecutiveSuccesses(monitor.ID, node.ID)
			if err != nil {
				continue
			}
			if successes >= monitor.RecoveryThreshold {
				recoveryCount++
			}
		}

		recoveryQuorumMet := consensus.EvaluateQuorum(monitor.QuorumType, monitor.QuorumN, recoveryCount, totalNodes)
		if recoveryQuorumMet {
			if err := a.incidentMgr.ResolveIncident(incident); err != nil {
				log.Printf("[consensus] error resolving incident %s: %v", incident.ID, err)
				return
			}
			a.alerter.SendRecovery(incident, monitor)
		}
	}
}

func (a *Agent) syncLoop(ctx context.Context) {
	// Initial sync
	a.syncMonitors()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.syncMonitors()
		}
	}
}

func (a *Agent) syncMonitors() {
	monitors, err := a.store.ListEnabledMonitors()
	if err != nil {
		log.Printf("[agent] error loading monitors: %v", err)
		return
	}
	a.scheduler.SyncMonitors(monitors)
}

// StartTime returns when the agent was started.
func (a *Agent) StartTime() time.Time {
	return a.startTime
}

// LastHeartbeat returns when the last heartbeat was sent/updated.
func (a *Agent) LastHeartbeat() time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastHeartbeat
}

// LastConfigSync returns when config was last synced.
func (a *Agent) LastConfigSync() time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastConfigSync
}

// ActiveMonitors returns the number of actively scheduled monitors.
func (a *Agent) ActiveMonitors() int {
	return a.scheduler.ActiveCount()
}

// Scheduler returns the agent's scheduler.
func (a *Agent) Scheduler() *Scheduler {
	return a.scheduler
}
