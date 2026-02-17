package api

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/pingmesh/pingmesh/internal/store"
)

// Server provides the HTTP API for both CLI commands and peer communication.
type Server struct {
	config     *config.Config
	store      store.Store
	cliServer  *http.Server
	peerServer *http.Server
}

// NewServer creates a new API server.
func NewServer(cfg *config.Config, st store.Store) *Server {
	s := &Server{
		config: cfg,
		store:  st,
	}

	// CLI API (localhost only)
	cliMux := http.NewServeMux()
	s.registerCLIRoutes(cliMux)
	s.cliServer = &http.Server{
		Handler:      cliMux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Peer API (mTLS)
	peerMux := http.NewServeMux()
	s.registerPeerRoutes(peerMux)
	s.peerServer = &http.Server{
		Handler:      peerMux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return s
}

// StartCLI starts the CLI API server on the configured address.
func (s *Server) StartCLI(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.config.CLIAddr)
	if err != nil {
		return err
	}
	log.Printf("[api] CLI API listening on %s", s.config.CLIAddr)

	go func() {
		<-ctx.Done()
		s.cliServer.Shutdown(context.Background())
	}()

	if err := s.cliServer.Serve(ln); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// StartPeer starts the peer API server with mTLS.
// TODO (M3): Add mTLS configuration
func (s *Server) StartPeer(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return err
	}
	log.Printf("[api] Peer API listening on %s", s.config.ListenAddr)

	go func() {
		<-ctx.Done()
		s.peerServer.Shutdown(context.Background())
	}()

	if err := s.peerServer.Serve(ln); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// readJSON decodes a JSON request body.
func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
