//go:build darwin || freebsd || linux || netbsd || openbsd
// +build darwin freebsd linux netbsd openbsd

package main

import (
	"runtime"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logger "github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
)

func main() {
	config, _ := appconfig.Config(false)
	// will use default when the value is less than one
	runtime.GOMAXPROCS(config.Agent.GoMaxProcForAgentWorker)

	// initialize logger
	log := logger.SSMLogger(true)
	defer log.Close()
	defer log.Flush()

	// parse input parameters
	parseFlags(log)

	// run agent
	run(log, true)
}
