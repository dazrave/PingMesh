package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/pingmesh/pingmesh/internal/model"
)

// PeerClient is an HTTP client for outbound peer communication.
type PeerClient struct {
	client *http.Client
}

// NewPeerClient creates a new PeerClient with sensible timeouts.
func NewPeerClient() *PeerClient {
	return &PeerClient{
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 5 * time.Second,
				}).DialContext,
			},
		},
	}
}

// SendHeartbeat sends a heartbeat to a peer node.
func (c *PeerClient) SendHeartbeat(addr string, hb *model.Heartbeat) error {
	return c.postJSON(addr, "/api/v1/peer/heartbeat", hb)
}

// PushResult sends a check result to the coordinator.
func (c *PeerClient) PushResult(addr string, result *model.CheckResult) error {
	return c.postJSON(addr, "/api/v1/peer/result", result)
}

// PushConfigSync sends a config sync to a peer node.
func (c *PeerClient) PushConfigSync(addr string, sync *model.ConfigSync) error {
	return c.postJSON(addr, "/api/v1/peer/config-sync", sync)
}

// Join sends a join request to the coordinator and returns the response.
func (c *PeerClient) Join(addr string, req *model.JoinRequest) (*model.JoinResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshalling join request: %w", err)
	}

	url := fmt.Sprintf("http://%s/api/v1/peer/join", addr)
	resp, err := c.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("sending join request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading join response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("join failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var joinResp model.JoinResponse
	if err := json.Unmarshal(respBody, &joinResp); err != nil {
		return nil, fmt.Errorf("decoding join response: %w", err)
	}

	return &joinResp, nil
}

// PullConfigSync fetches the current config from the coordinator (pull-based sync).
func (c *PeerClient) PullConfigSync(addr string) (*model.ConfigSync, error) {
	url := fmt.Sprintf("http://%s/api/v1/peer/config-sync", addr)
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET config-sync: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET config-sync returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var sync model.ConfigSync
	if err := json.NewDecoder(resp.Body).Decode(&sync); err != nil {
		return nil, fmt.Errorf("decoding config-sync: %w", err)
	}
	return &sync, nil
}

func (c *PeerClient) postJSON(addr, path string, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshalling request: %w", err)
	}

	url := fmt.Sprintf("http://%s%s", addr, path)
	resp, err := c.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST %s returned HTTP %d: %s", path, resp.StatusCode, string(respBody))
	}

	return nil
}
