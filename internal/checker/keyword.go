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

// KeywordChecker performs HTTP requests and checks for a keyword in the response body.
type KeywordChecker struct{}

func (c *KeywordChecker) Type() model.CheckType {
	return model.CheckHTTPKeyword
}

func (c *KeywordChecker) Check(ctx context.Context, monitor *model.Monitor) (*Result, error) {
	timeout := time.Duration(monitor.TimeoutMS) * time.Millisecond

	port := monitor.Port
	if port == 0 {
		port = 80
	}

	url := fmt.Sprintf("http://%s:%d", monitor.Target, port)

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
			DisableKeepAlives: true,
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

	// Read body (limit to 1MB to prevent memory issues)
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return &Result{
			Status:     model.StatusDown,
			LatencyMS:  latency,
			StatusCode: resp.StatusCode,
			Error:      fmt.Sprintf("reading body: %v", err),
		}, nil
	}

	body := string(bodyBytes)
	keywordFound := strings.Contains(body, monitor.ExpectedKeyword)

	result := &Result{
		Status:     model.StatusUp,
		LatencyMS:  latency,
		StatusCode: resp.StatusCode,
		Details: map[string]any{
			"status_code":   resp.StatusCode,
			"keyword_found": keywordFound,
			"body_length":   len(bodyBytes),
		},
	}

	if !keywordFound {
		result.Status = model.StatusDown
		result.Error = fmt.Sprintf("keyword %q not found in response", monitor.ExpectedKeyword)
	}

	if resp.StatusCode >= 400 {
		result.Status = model.StatusDown
		if result.Error == "" {
			result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
	}

	return result, nil
}
