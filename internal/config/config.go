package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultDataDir    = "/var/lib/pingmesh"
	DefaultListenAddr = "0.0.0.0:7433"
	DefaultCLIAddr    = "127.0.0.1:7434"
	ConfigFileName    = "config.json"
)

// Config holds all configuration for a PingMesh node.
type Config struct {
	NodeID     string `json:"node_id"`
	NodeName   string `json:"node_name"`
	Role       string `json:"role"` // "coordinator" or "node"
	DataDir    string `json:"data_dir"`
	ListenAddr string `json:"listen_addr"` // mTLS peer API
	CLIAddr    string `json:"cli_addr"`    // local CLI API (localhost only)

	Coordinator *CoordinatorConfig `json:"coordinator,omitempty"`
	TLS         *TLSConfig         `json:"tls,omitempty"`
}

// CoordinatorConfig holds coordinator-specific settings.
type CoordinatorConfig struct {
	Address string `json:"address"` // coordinator address for nodes to connect to
}

// TLSConfig holds paths to TLS certificates.
type TLSConfig struct {
	CAPath   string `json:"ca_path"`
	CertPath string `json:"cert_path"`
	KeyPath  string `json:"key_path"`
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		DataDir:    DefaultDataDir,
		ListenAddr: DefaultListenAddr,
		CLIAddr:    DefaultCLIAddr,
	}
}

// Load reads configuration from the data directory.
func Load(dataDir string) (*Config, error) {
	path := filepath.Join(dataDir, ConfigFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no configuration found at %s (run 'pingmesh init' or 'pingmesh join' first)", path)
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.DataDir == "" {
		cfg.DataDir = dataDir
	}

	return &cfg, nil
}

// Save writes configuration to the data directory.
func (c *Config) Save() error {
	if err := os.MkdirAll(c.DataDir, 0750); err != nil {
		return fmt.Errorf("creating data directory: %w", err)
	}

	path := filepath.Join(c.DataDir, ConfigFileName)

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// DBPath returns the path to the SQLite database.
func (c *Config) DBPath() string {
	return filepath.Join(c.DataDir, "pingmesh.db")
}

// CertsDir returns the path to the certificates directory.
func (c *Config) CertsDir() string {
	return filepath.Join(c.DataDir, "certs")
}
