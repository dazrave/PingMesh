package api

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/pingmesh/pingmesh/internal/cluster"
	"github.com/pingmesh/pingmesh/internal/config"
	"github.com/pingmesh/pingmesh/internal/logbuf"
	"github.com/pingmesh/pingmesh/internal/store"
	"github.com/pingmesh/pingmesh/internal/web"
)

// AgentInfo provides runtime metrics from the agent without circular imports.
type AgentInfo interface {
	StartTime() time.Time
	LastHeartbeat() time.Time
	LastConfigSync() time.Time
	ActiveMonitors() int
}

// AlertDispatcher sends alerts and test notifications.
type AlertDispatcher interface {
	SendTest(channelID string) error
}

// ServerOption configures the Server.
type ServerOption func(*Server)

// WithLogBuffer attaches a log ring buffer for the /api/v1/logs endpoint.
func WithLogBuffer(buf *logbuf.Buffer) ServerOption {
	return func(s *Server) { s.logBuf = buf }
}

// WithAgentInfo attaches agent runtime info for the health endpoint.
func WithAgentInfo(ai AgentInfo) ServerOption {
	return func(s *Server) { s.agentInfo = ai }
}

// WithAlertDispatcher attaches an alert dispatcher for test-alert endpoint.
func WithAlertDispatcher(ad AlertDispatcher) ServerOption {
	return func(s *Server) { s.alertDispatcher = ad }
}

// Server provides the HTTP API for both CLI commands and peer communication.
type Server struct {
	config     *config.Config
	store      store.Store
	clusterMgr *cluster.Manager
	logBuf          *logbuf.Buffer
	agentInfo       AgentInfo
	alertDispatcher AlertDispatcher
	cliServer       *http.Server
	peerServer      *http.Server
}

// NewServer creates a new API server.
func NewServer(cfg *config.Config, st store.Store, opts ...ServerOption) *Server {
	s := &Server{
		config:     cfg,
		store:      st,
		clusterMgr: cluster.NewManager(cfg, st),
	}

	for _, opt := range opts {
		opt(s)
	}

	// CLI API (localhost only)
	cliMux := http.NewServeMux()
	s.registerCLIRoutes(cliMux)

	// Serve embedded web dashboard (catch-all — API routes take priority)
	if staticFS, err := web.StaticFS(); err == nil {
		cliMux.Handle("/", spaHandler(staticFS))
	}

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

// spaHandler serves static files from the embedded FS. For paths that don't
// match a real file, it serves index.html to support client-side routing.
func spaHandler(root fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file directly
		path := r.URL.Path
		if path == "/" {
			path = "index.html"
		} else {
			// Strip leading slash for fs.Open
			path = path[1:]
		}

		// Check if the file exists in the embedded FS
		if _, err := fs.Stat(root, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fall back to index.html for SPA client-side routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
