package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/pingmesh/pingmesh/internal/checker"
	"github.com/pingmesh/pingmesh/internal/model"
	"github.com/pingmesh/pingmesh/internal/store"
)

// Scheduler manages periodic execution of monitoring checks.
type Scheduler struct {
	store  store.Store
	nodeID string

	mu       sync.Mutex
	timers   map[string]*time.Ticker // monitor ID -> ticker
	cancels  map[string]context.CancelFunc
	running  map[string]bool // track if a check is currently executing
}

// NewScheduler creates a new check scheduler.
func NewScheduler(st store.Store, nodeID string) *Scheduler {
	return &Scheduler{
		store:   st,
		nodeID:  nodeID,
		timers:  make(map[string]*time.Ticker),
		cancels: make(map[string]context.CancelFunc),
		running: make(map[string]bool),
	}
}

// SyncMonitors updates the scheduler to match the current set of enabled monitors.
func (s *Scheduler) SyncMonitors(monitors []model.Monitor) {
	s.mu.Lock()
	defer s.mu.Unlock()

	active := make(map[string]bool)
	for _, m := range monitors {
		active[m.ID] = true
		if _, exists := s.timers[m.ID]; !exists {
			s.startMonitor(m)
		}
	}

	// Stop monitors that are no longer active
	for id := range s.timers {
		if !active[id] {
			s.stopMonitor(id)
		}
	}
}

// Stop halts all scheduled checks.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id := range s.timers {
		s.stopMonitor(id)
	}
}

func (s *Scheduler) startMonitor(m model.Monitor) {
	interval := time.Duration(m.IntervalMS) * time.Millisecond
	if interval < time.Second {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	ctx, cancel := context.WithCancel(context.Background())

	s.timers[m.ID] = ticker
	s.cancels[m.ID] = cancel

	go s.runLoop(ctx, m.ID, ticker)
}

func (s *Scheduler) stopMonitor(id string) {
	if ticker, ok := s.timers[id]; ok {
		ticker.Stop()
		delete(s.timers, id)
	}
	if cancel, ok := s.cancels[id]; ok {
		cancel()
		delete(s.cancels, id)
	}
	delete(s.running, id)
}

func (s *Scheduler) runLoop(ctx context.Context, monitorID string, ticker *time.Ticker) {
	// Run immediately on start
	s.executeCheck(ctx, monitorID)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.executeCheck(ctx, monitorID)
		}
	}
}

func (s *Scheduler) executeCheck(ctx context.Context, monitorID string) {
	// Skip if previous check still running
	s.mu.Lock()
	if s.running[monitorID] {
		s.mu.Unlock()
		log.Printf("[scheduler] skipping check for %s: previous still running", monitorID)
		return
	}
	s.running[monitorID] = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running[monitorID] = false
		s.mu.Unlock()
	}()

	monitor, err := s.store.GetMonitor(monitorID)
	if err != nil || monitor == nil {
		log.Printf("[scheduler] monitor %s not found, skipping", monitorID)
		return
	}

	c, err := checker.Get(monitor.CheckType)
	if err != nil {
		log.Printf("[scheduler] no checker for %s: %v", monitor.CheckType, err)
		return
	}

	checkCtx, cancel := context.WithTimeout(ctx, time.Duration(monitor.TimeoutMS)*time.Millisecond)
	defer cancel()

	var lastResult *checker.Result
	var lastErr error
	attempts := monitor.Retries
	if attempts < 1 {
		attempts = 1
	}

	for i := 0; i < attempts; i++ {
		lastResult, lastErr = c.Check(checkCtx, monitor)
		if lastErr != nil {
			log.Printf("[scheduler] check error for %s (attempt %d): %v", monitorID, i+1, lastErr)
			continue
		}
		if lastResult.Status == model.StatusUp {
			break
		}
	}

	if lastResult == nil {
		lastResult = &checker.Result{
			Status: model.StatusDown,
			Error:  "all attempts failed",
		}
		if lastErr != nil {
			lastResult.Error = lastErr.Error()
		}
	}

	result := &model.CheckResult{
		MonitorID:  monitorID,
		NodeID:     s.nodeID,
		Status:     lastResult.Status,
		LatencyMS:  lastResult.LatencyMS,
		StatusCode: lastResult.StatusCode,
		Error:      lastResult.Error,
		Timestamp:  time.Now().UnixMilli(),
	}

	if err := s.store.InsertCheckResult(result); err != nil {
		log.Printf("[scheduler] failed to store result for %s: %v", monitorID, err)
	}

	log.Printf("[check] %s → %s (%.1fms)", monitor.Name, result.Status, result.LatencyMS)
}

// ActiveCount returns the number of actively scheduled monitors.
func (s *Scheduler) ActiveCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.timers)
}
