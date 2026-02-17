package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pingmesh/pingmesh/internal/model"
)

// HTTPChecker performs HTTP/HTTPS status checks.
type HTTPChecker struct {
	checkType model.CheckType
}

func (c *HTTPChecker) Type() model.CheckType {
	return c.checkType
}

func (c *HTTPChecker) Check(ctx context.Context, monitor *model.Monitor) (*Result, error) {
	timeout := time.Duration(monitor.TimeoutMS) * time.Millisecond

	scheme := "http"
	if c.checkType == model.CheckHTTPS {
		scheme = "https"
	}

	// Strip scheme if user included it in the target (e.g. "https://example.com")
	target := monitor.Target
	target = strings.TrimPrefix(target, "https://")
	target = strings.TrimPrefix(target, "http://")
	target = strings.TrimRight(target, "/")

	port := monitor.Port
	if port == 0 {
		if scheme == "https" {
			port = 443
		} else {
			port = 80
		}
	}

	url := fmt.Sprintf("%s://%s:%d", scheme, target, port)

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
			DisableKeepAlives: true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return &Result{
			Status: model.StatusDown,
			Error:  fmt.Sprintf("creating request: %v", err),
		}, nil
	}
	req.Header.Set("User-Agent", "PingMesh/1.0")

	start := time.Now()
	resp, err := client.Do(req)
	latency := float64(time.Since(start).Microseconds()) / 1000.0

	if err != nil {
		return &Result{
			Status:    model.StatusDown,
			LatencyMS: latency,
			Error:     fmt.Sprintf("request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	result := &Result{
		Status:     model.StatusUp,
		LatencyMS:  latency,
		StatusCode: resp.StatusCode,
		Details: map[string]any{
			"status_code": resp.StatusCode,
			"protocol":    resp.Proto,
		},
	}

	// Check expected status code
	if monitor.ExpectedStatus > 0 && resp.StatusCode != monitor.ExpectedStatus {
		result.Status = model.StatusDown
		result.Error = fmt.Sprintf("expected status %d, got %d", monitor.ExpectedStatus, resp.StatusCode)
	} else if monitor.ExpectedStatus == 0 && resp.StatusCode >= 400 {
		result.Status = model.StatusDown
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	// Check TLS certificate expiry for HTTPS
	if c.checkType == model.CheckHTTPS && resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		cert := resp.TLS.PeerCertificates[0]
		daysUntilExpiry := time.Until(cert.NotAfter).Hours() / 24
		result.Details["tls_expiry_days"] = int(daysUntilExpiry)
		result.Details["tls_issuer"] = cert.Issuer.CommonName
		result.Details["tls_subject"] = cert.Subject.CommonName

		if daysUntilExpiry < 7 {
			result.Status = model.StatusDegraded
			result.Error = fmt.Sprintf("TLS certificate expires in %d days", int(daysUntilExpiry))
		}
	}

	return result, nil
}
