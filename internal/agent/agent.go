package agent

import (
	"context"
	"log"
	"time"

	"github.com/pingmesh/pingmesh/internal/checker"
	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/pingmesh/pingmesh/internal/store"
)

// Agent is the main runtime that coordinates check scheduling, cluster communication, and API serving.
type Agent struct {
	config    *config.Config
	store     store.Store
	scheduler *Scheduler
	startTime time.Time
}

// New creates a new Agent instance.
func New(cfg *config.Config, st store.Store) *Agent {
	return &Agent{
		config:    cfg,
		store:     st,
		scheduler: NewScheduler(st, cfg.NodeID),
		startTime: time.Now(),
	}
}

// Run starts the agent and blocks until the context is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	log.Printf("[agent] starting node %s (%s) role=%s", a.config.NodeID, a.config.NodeName, a.config.Role)

	// Register all check types
	checker.RegisterAll()

	// Start the monitor sync loop
	go a.syncLoop(ctx)

	// TODO (M3): Start cluster heartbeat loop
	// TODO (M3): Start peer API server
	// TODO (M4): Start consensus evaluator

	<-ctx.Done()
	log.Println("[agent] shutting down...")
	a.scheduler.Stop()
	return nil
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

// Scheduler returns the agent's scheduler.
func (a *Agent) Scheduler() *Scheduler {
	return a.scheduler
}
