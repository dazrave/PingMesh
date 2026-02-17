package checker

import (
	"context"
	"fmt"
	"time"

	"github.com/pingmesh/pingmesh/internal/model"
	probing "github.com/prometheus-community/pro-bing"
)

// ICMPChecker performs ICMP ping checks.
type ICMPChecker struct{}

func (c *ICMPChecker) Type() model.CheckType {
	return model.CheckICMP
}

func (c *ICMPChecker) Check(ctx context.Context, monitor *model.Monitor) (*Result, error) {
	timeout := time.Duration(monitor.TimeoutMS) * time.Millisecond

	pinger, err := probing.NewPinger(monitor.Target)
	if err != nil {
		return &Result{
			Status: model.StatusDown,
			Error:  fmt.Sprintf("creating pinger: %v", err),
		}, nil
	}

	pinger.Count = 1
	pinger.Timeout = timeout
	pinger.SetPrivileged(false) // Use UDP sockets (unprivileged)

	err = pinger.RunWithContext(ctx)
	if err != nil {
		return &Result{
			Status: model.StatusDown,
			Error:  fmt.Sprintf("ping failed: %v", err),
		}, nil
	}

	stats := pinger.Statistics()
	if stats.PacketsRecv == 0 {
		return &Result{
			Status:    model.StatusDown,
			LatencyMS: 0,
			Error:     "no reply received",
			Details: map[string]any{
				"packets_sent": stats.PacketsSent,
				"packets_recv": stats.PacketsRecv,
			},
		}, nil
	}

	return &Result{
		Status:    model.StatusUp,
		LatencyMS: float64(stats.AvgRtt.Microseconds()) / 1000.0,
		Details: map[string]any{
			"packets_sent": stats.PacketsSent,
			"packets_recv": stats.PacketsRecv,
			"packet_loss":  stats.PacketLoss,
			"min_rtt_ms":   float64(stats.MinRtt.Microseconds()) / 1000.0,
			"max_rtt_ms":   float64(stats.MaxRtt.Microseconds()) / 1000.0,
			"avg_rtt_ms":   float64(stats.AvgRtt.Microseconds()) / 1000.0,
		},
	}, nil
}
