package checker

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/pingmesh/pingmesh/internal/model"
)

// TCPChecker performs TCP port connectivity checks.
type TCPChecker struct{}

func (c *TCPChecker) Type() model.CheckType {
	return model.CheckTCP
}

func (c *TCPChecker) Check(ctx context.Context, monitor *model.Monitor) (*Result, error) {
	timeout := time.Duration(monitor.TimeoutMS) * time.Millisecond
	address := fmt.Sprintf("%s:%d", monitor.Target, monitor.Port)

	start := time.Now()

	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	latency := float64(time.Since(start).Microseconds()) / 1000.0

	if err != nil {
		return &Result{
			Status:    model.StatusDown,
			LatencyMS: latency,
			Error:     fmt.Sprintf("tcp connect failed: %v", err),
		}, nil
	}
	conn.Close()

	return &Result{
		Status:    model.StatusUp,
		LatencyMS: latency,
	}, nil
}
