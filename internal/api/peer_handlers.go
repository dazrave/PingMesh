package api

import (
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/pingmesh/pingmesh/internal/cluster"
	"github.com/pingmesh/pingmesh/internal/model"
)

func (s *Server) registerPeerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/peer/check", s.handlePeerCheck)
	mux.HandleFunc("POST /api/v1/peer/heartbeat", s.handlePeerHeartbeat)
	mux.HandleFunc("POST /api/v1/peer/config-sync", s.handlePeerConfigSync)
	mux.HandleFunc("POST /api/v1/peer/join", s.handlePeerJoin)
	mux.HandleFunc("POST /api/v1/peer/result", s.handlePeerResult)
}

// handlePeerCheck handles a request from a peer to execute a check.
func (s *Server) handlePeerCheck(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "peer check not yet implemented")
}

// handlePeerHeartbeat handles a heartbeat from a peer node.
func (s *Server) handlePeerHeartbeat(w http.ResponseWriter, r *http.Request) {
	var hb model.Heartbeat
	if err := readJSON(r, &hb); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := s.clusterMgr.UpdateHeartbeat(hb.NodeID); err != nil {
		log.Printf("[peer] heartbeat error for node %s: %v", hb.NodeID, err)
		writeError(w, http.StatusInternalServerError, "heartbeat update failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handlePeerConfigSync handles configuration sync from coordinator.
func (s *Server) handlePeerConfigSync(w http.ResponseWriter, r *http.Request) {
	var sync model.ConfigSync
	if err := readJSON(r, &sync); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Sync monitors
	for i := range sync.Monitors {
		m := &sync.Monitors[i]
		existing, err := s.store.GetMonitor(m.ID)
		if err != nil {
			log.Printf("[peer] config-sync: error checking monitor %s: %v", m.ID, err)
			continue
		}
		if existing != nil {
			if err := s.store.UpdateMonitor(m); err != nil {
				log.Printf("[peer] config-sync: error updating monitor %s: %v", m.ID, err)
			}
		} else {
			if err := s.store.CreateMonitor(m); err != nil {
				log.Printf("[peer] config-sync: error creating monitor %s: %v", m.ID, err)
			}
		}
	}

	// Sync nodes
	for i := range sync.Nodes {
		n := &sync.Nodes[i]
		existing, err := s.store.GetNode(n.ID)
		if err != nil {
			log.Printf("[peer] config-sync: error checking node %s: %v", n.ID, err)
			continue
		}
		if existing != nil {
			if err := s.store.UpdateNode(n); err != nil {
				log.Printf("[peer] config-sync: error updating node %s: %v", n.ID, err)
			}
		} else {
			if err := s.store.CreateNode(n); err != nil {
				log.Printf("[peer] config-sync: error creating node %s: %v", n.ID, err)
			}
		}
	}

	log.Printf("[peer] config-sync applied: %d monitors, %d nodes", len(sync.Monitors), len(sync.Nodes))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handlePeerJoin handles a join request from a new node.
func (s *Server) handlePeerJoin(w http.ResponseWriter, r *http.Request) {
	if s.config.Role != model.RoleCoordinator {
		writeError(w, http.StatusForbidden, "only the coordinator can accept join requests")
		return
	}

	var req model.JoinRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Validate token
	valid, err := cluster.ValidateJoinToken(s.store, req.Secret)
	if err != nil {
		log.Printf("[peer] join: token validation error: %v", err)
		writeError(w, http.StatusInternalServerError, "token validation failed")
		return
	}
	if !valid {
		writeError(w, http.StatusUnauthorized, "invalid or expired join token")
		return
	}

	// Generate node UUID
	nodeID := uuid.New().String()

	// Resolve node address: if ListenAddr has 0.0.0.0, replace with remote IP
	listenAddr := req.ListenAddr
	listenHost, listenPort, _ := net.SplitHostPort(listenAddr)
	if listenHost == "0.0.0.0" || listenHost == "" {
		remoteHost, _, _ := net.SplitHostPort(r.RemoteAddr)
		listenAddr = net.JoinHostPort(remoteHost, listenPort)
	}

	// Generate cert with the resolved address
	addresses := []string{"127.0.0.1"}
	if host, _, err := net.SplitHostPort(listenAddr); err == nil {
		if ip := net.ParseIP(host); ip != nil && !ip.IsLoopback() {
			addresses = append(addresses, host)
		}
	}

	certsDir := s.config.CertsDir()
	certPEM, keyPEM, err := cluster.GenerateNodeCertPEM(certsDir, nodeID, addresses)
	if err != nil {
		log.Printf("[peer] join: cert generation error: %v", err)
		writeError(w, http.StatusInternalServerError, "certificate generation failed")
		return
	}

	// Read CA cert
	caCertPEM, err := os.ReadFile(certsDir + "/ca.crt")
	if err != nil {
		log.Printf("[peer] join: error reading CA cert: %v", err)
		writeError(w, http.StatusInternalServerError, "reading CA cert failed")
		return
	}

	// Create node record
	now := time.Now().UnixMilli()
	node := &model.Node{
		ID:        nodeID,
		Name:      req.Name,
		Address:   listenAddr,
		Role:      model.RoleNode,
		Status:    model.NodeOnline,
		LastSeen:  now,
		CreatedAt: now,
	}
	if err := s.store.CreateNode(node); err != nil {
		log.Printf("[peer] join: error creating node record: %v", err)
		writeError(w, http.StatusInternalServerError, "creating node record failed")
		return
	}

	log.Printf("[peer] node %s (%s) joined at %s", nodeID, req.Name, listenAddr)

	resp := model.JoinResponse{
		NodeID:        nodeID,
		CACert:        string(caCertPEM),
		NodeCert:      string(certPEM),
		NodeKey:       string(keyPEM),
		CoordinatorID: s.config.NodeID,
	}
	writeJSON(w, http.StatusOK, resp)
}

// handlePeerResult handles a check result pushed from a peer node.
func (s *Server) handlePeerResult(w http.ResponseWriter, r *http.Request) {
	var result model.CheckResult
	if err := readJSON(r, &result); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := s.store.InsertCheckResult(&result); err != nil {
		log.Printf("[peer] result: error storing result: %v", err)
		writeError(w, http.StatusInternalServerError, "storing result failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
