package api

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/pingmesh/pingmesh/internal/model"
)

func (s *Server) registerCLIRoutes(mux *http.ServeMux) {
	// Node endpoints
	mux.HandleFunc("GET /api/v1/nodes", s.handleListNodes)
	mux.HandleFunc("GET /api/v1/nodes/{id}", s.handleGetNode)
	mux.HandleFunc("DELETE /api/v1/nodes/{id}", s.handleDeleteNode)

	// Monitor endpoints
	mux.HandleFunc("GET /api/v1/monitors", s.handleListMonitors)
	mux.HandleFunc("POST /api/v1/monitors", s.handleCreateMonitor)
	mux.HandleFunc("GET /api/v1/monitors/{id}", s.handleGetMonitor)
	mux.HandleFunc("PUT /api/v1/monitors/{id}", s.handleUpdateMonitor)
	mux.HandleFunc("DELETE /api/v1/monitors/{id}", s.handleDeleteMonitor)

	// Status & incidents
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	mux.HandleFunc("GET /api/v1/incidents", s.handleListIncidents)

	// History
	mux.HandleFunc("GET /api/v1/history", s.handleHistory)

	// Health
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)

	// Logs
	mux.HandleFunc("GET /api/v1/logs", s.handleLogs)

	// Peer connectivity test
	mux.HandleFunc("GET /api/v1/test-peer", s.handleTestPeer)
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.store.ListNodes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if nodes == nil {
		nodes = []model.Node{}
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	node, err := s.store.GetNode(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteNode(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleListMonitors(w http.ResponseWriter, r *http.Request) {
	group := r.URL.Query().Get("group")
	monitors, err := s.store.ListMonitors(group)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if monitors == nil {
		monitors = []model.Monitor{}
	}
	writeJSON(w, http.StatusOK, monitors)
}

func (s *Server) handleCreateMonitor(w http.ResponseWriter, r *http.Request) {
	var m model.Monitor
	if err := readJSON(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	now := time.Now().UnixMilli()
	m.ID = uuid.New().String()
	m.CreatedAt = now
	m.UpdatedAt = now

	// Set defaults
	if m.IntervalMS == 0 {
		m.IntervalMS = 60000
	}
	if m.TimeoutMS == 0 {
		m.TimeoutMS = 5000
	}
	if m.Retries == 0 {
		m.Retries = 1
	}
	if m.FailureThreshold == 0 {
		m.FailureThreshold = 3
	}
	if m.RecoveryThreshold == 0 {
		m.RecoveryThreshold = 2
	}
	if m.QuorumType == "" {
		m.QuorumType = "majority"
	}
	if m.CooldownMS == 0 {
		m.CooldownMS = 300000
	}
	m.Enabled = true

	if err := s.store.CreateMonitor(&m); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handleGetMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	monitor, err := s.store.GetMonitor(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if monitor == nil {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	}
	writeJSON(w, http.StatusOK, monitor)
}

func (s *Server) handleUpdateMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	existing, err := s.store.GetMonitor(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	}

	var updates model.Monitor
	if err := readJSON(r, &updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Apply non-zero updates
	if updates.Name != "" {
		existing.Name = updates.Name
	}
	if updates.Target != "" {
		existing.Target = updates.Target
	}
	if updates.Port != 0 {
		existing.Port = updates.Port
	}
	if updates.IntervalMS != 0 {
		existing.IntervalMS = updates.IntervalMS
	}
	if updates.TimeoutMS != 0 {
		existing.TimeoutMS = updates.TimeoutMS
	}
	if updates.GroupName != "" {
		existing.GroupName = updates.GroupName
	}
	existing.UpdatedAt = time.Now().UnixMilli()

	if err := s.store.UpdateMonitor(existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteMonitor(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	nodes, _ := s.store.ListNodes()
	monitors, _ := s.store.ListMonitors("")
	incidents, _ := s.store.ListIncidents(true)

	if nodes == nil {
		nodes = []model.Node{}
	}
	if incidents == nil {
		incidents = []model.Incident{}
	}

	status := model.ClusterStatus{
		NodeID:          s.config.NodeID,
		Role:            s.config.Role,
		Nodes:           nodes,
		MonitorCount:    len(monitors),
		ActiveIncidents: incidents,
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleListIncidents(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active") == "true"
	incidents, err := s.store.ListIncidents(activeOnly)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if incidents == nil {
		incidents = []model.Incident{}
	}
	writeJSON(w, http.StatusOK, incidents)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	monitorID := r.URL.Query().Get("monitor")
	nodeID := r.URL.Query().Get("node")

	var since int64
	if s := r.URL.Query().Get("since"); s != "" {
		since, _ = strconv.ParseInt(s, 10, 64)
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	results, err := s.store.ListCheckResults(monitorID, nodeID, since, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []model.CheckResult{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	health := model.HealthInfo{
		NodeID:        s.config.NodeID,
		Name:          s.config.NodeName,
		Role:          s.config.Role,
		GoVersion:     runtime.Version(),
		NumGoroutines: runtime.NumGoroutine(),
		MemoryMB:      float64(memStats.Alloc) / 1024 / 1024,
	}

	// DB size
	if fi, err := os.Stat(s.config.DBPath()); err == nil {
		health.DBSizeMB = float64(fi.Size()) / 1024 / 1024
	}

	// Agent-provided info (uptime, monitors, timestamps)
	if ai := s.agentInfo; ai != nil {
		health.Uptime = time.Since(ai.StartTime()).Truncate(time.Second).String()
		health.ActiveMonitors = ai.ActiveMonitors()
		if t := ai.LastHeartbeat(); !t.IsZero() {
			health.LastHeartbeat = t.Format(time.RFC3339)
		}
		if t := ai.LastConfigSync(); !t.IsZero() {
			health.LastConfigSync = t.Format(time.RFC3339)
		}
	}

	// Coordinator info
	if s.config.Coordinator != nil {
		health.Coordinator = s.config.Coordinator.Address
	}

	// Peer connectivity
	health.Peers = s.probePeers("")

	writeJSON(w, http.StatusOK, health)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if s.logBuf == nil {
		writeError(w, http.StatusServiceUnavailable, "log buffer not available")
		return
	}

	n := 100
	if v := r.URL.Query().Get("lines"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			n = parsed
		}
	}

	entries := s.logBuf.Last(n)
	if entries == nil {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleTestPeer(w http.ResponseWriter, r *http.Request) {
	filterNode := r.URL.Query().Get("node")
	peers := s.probePeers(filterNode)
	writeJSON(w, http.StatusOK, peers)
}

// probePeers TCP-dials each peer and returns reachability status.
// If filterNode is non-empty, only that node ID is tested.
func (s *Server) probePeers(filterNode string) []model.PeerStatus {
	nodes, err := s.store.ListNodes()
	if err != nil {
		return nil
	}

	var peers []model.PeerStatus
	for _, n := range nodes {
		if n.ID == s.config.NodeID {
			continue
		}
		if filterNode != "" && n.ID != filterNode {
			continue
		}

		ps := model.PeerStatus{
			NodeID:  n.ID,
			Name:    n.Name,
			Address: n.Address,
			Status:  n.Status,
		}

		start := time.Now()
		conn, err := net.DialTimeout("tcp", n.Address, 3*time.Second)
		if err != nil {
			ps.Reachable = false
			ps.Error = fmt.Sprintf("dial: %v", err)
		} else {
			ps.Reachable = true
			ps.LatencyMS = float64(time.Since(start).Microseconds()) / 1000
			conn.Close()
		}

		peers = append(peers, ps)
	}

	if peers == nil {
		peers = []model.PeerStatus{}
	}
	return peers
}
