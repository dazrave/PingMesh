package api

import (
	"net/http"
)

func (s *Server) registerPeerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/peer/check", s.handlePeerCheck)
	mux.HandleFunc("POST /api/v1/peer/heartbeat", s.handlePeerHeartbeat)
	mux.HandleFunc("POST /api/v1/peer/config-sync", s.handlePeerConfigSync)
	mux.HandleFunc("POST /api/v1/peer/join", s.handlePeerJoin)
}

// handlePeerCheck handles a request from a peer to execute a check.
// TODO (M3): Implement peer check execution
func (s *Server) handlePeerCheck(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "peer check not yet implemented")
}

// handlePeerHeartbeat handles a heartbeat from a peer node.
// TODO (M3): Implement heartbeat processing
func (s *Server) handlePeerHeartbeat(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "heartbeat not yet implemented")
}

// handlePeerConfigSync handles configuration sync from coordinator.
// TODO (M3): Implement config sync
func (s *Server) handlePeerConfigSync(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "config sync not yet implemented")
}

// handlePeerJoin handles a join request from a new node.
// TODO (M3): Implement join handling with token validation and cert issuance
func (s *Server) handlePeerJoin(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "join not yet implemented")
}
