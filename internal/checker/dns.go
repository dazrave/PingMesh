package checker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/pingmesh/pingmesh/internal/model"
)

// DNSChecker performs DNS resolution checks.
type DNSChecker struct{}

func (c *DNSChecker) Type() model.CheckType {
	return model.CheckDNS
}

func (c *DNSChecker) Check(ctx context.Context, monitor *model.Monitor) (*Result, error) {
	timeout := time.Duration(monitor.TimeoutMS) * time.Millisecond

	recordType := dns.TypeA
	switch strings.ToUpper(monitor.DNSRecordType) {
	case "AAAA":
		recordType = dns.TypeAAAA
	case "CNAME":
		recordType = dns.TypeCNAME
	case "MX":
		recordType = dns.TypeMX
	case "TXT":
		recordType = dns.TypeTXT
	}

	target := monitor.Target
	if !strings.HasSuffix(target, ".") {
		target += "."
	}

	m := new(dns.Msg)
	m.SetQuestion(target, recordType)
	m.RecursionDesired = true

	server := "8.8.8.8:53"
	if monitor.Port > 0 {
		server = fmt.Sprintf("%s:%d", monitor.Target, monitor.Port)
		// If port is set, the target is the DNS server, so we need the actual query target
		// For simplicity in MVP, we use the target as both server and query
	}

	client := &dns.Client{
		Timeout: timeout,
	}

	start := time.Now()
	resp, _, err := client.ExchangeContext(ctx, m, server)
	latency := float64(time.Since(start).Microseconds()) / 1000.0

	if err != nil {
		return &Result{
			Status:    model.StatusDown,
			LatencyMS: latency,
			Error:     fmt.Sprintf("dns query failed: %v", err),
		}, nil
	}

	if resp.Rcode != dns.RcodeSuccess {
		return &Result{
			Status:    model.StatusDown,
			LatencyMS: latency,
			Error:     fmt.Sprintf("dns error: %s", dns.RcodeToString[resp.Rcode]),
			Details: map[string]any{
				"rcode": dns.RcodeToString[resp.Rcode],
			},
		}, nil
	}

	var answers []string
	for _, rr := range resp.Answer {
		switch v := rr.(type) {
		case *dns.A:
			answers = append(answers, v.A.String())
		case *dns.AAAA:
			answers = append(answers, v.AAAA.String())
		case *dns.CNAME:
			answers = append(answers, v.Target)
		case *dns.MX:
			answers = append(answers, fmt.Sprintf("%d %s", v.Preference, v.Mx))
		case *dns.TXT:
			answers = append(answers, strings.Join(v.Txt, " "))
		}
	}

	result := &Result{
		Status:    model.StatusUp,
		LatencyMS: latency,
		Details: map[string]any{
			"answers":      answers,
			"answer_count": len(answers),
		},
	}

	// Check expected answer if configured
	if monitor.DNSExpected != "" {
		found := false
		for _, answer := range answers {
			if answer == monitor.DNSExpected {
				found = true
				break
			}
		}
		if !found {
			result.Status = model.StatusDown
			result.Error = fmt.Sprintf("expected answer %q not found in %v", monitor.DNSExpected, answers)
		}
	}

	return result, nil
}
