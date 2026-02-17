package checker

import (
	"context"
	"fmt"

	"github.com/pingmesh/pingmesh/internal/model"
)

// Result holds the outcome of a check execution.
type Result struct {
	Status     model.CheckStatus
	LatencyMS  float64
	StatusCode int
	Error      string
	Details    map[string]any
}

// Checker is the interface that all check types implement.
type Checker interface {
	Check(ctx context.Context, monitor *model.Monitor) (*Result, error)
	Type() model.CheckType
}

// registry maps check types to their implementations.
var registry = map[model.CheckType]Checker{}

// Register adds a checker to the registry.
func Register(c Checker) {
	registry[c.Type()] = c
}

// Get returns the checker for the given type.
func Get(checkType model.CheckType) (Checker, error) {
	c, ok := registry[checkType]
	if !ok {
		return nil, fmt.Errorf("unknown check type: %s", checkType)
	}
	return c, nil
}

// RegisterAll registers all built-in checker implementations.
func RegisterAll() {
	Register(&ICMPChecker{})
	Register(&TCPChecker{})
	Register(&HTTPChecker{checkType: model.CheckHTTP})
	Register(&HTTPChecker{checkType: model.CheckHTTPS})
	Register(&DNSChecker{})
	Register(&KeywordChecker{})
}
