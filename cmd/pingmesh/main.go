package main

import (
	"github.com/pingmesh/pingmesh/internal/cli"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	cli.Execute()
}
