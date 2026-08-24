//go:build darwin || freebsd || linux || netbsd || openbsd
// +build darwin freebsd linux netbsd openbsd

package main

import (
	"runtime"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logger "github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

func main() {
	config, _ := appconfig.Config(false)
	// will use default when the value is less than one
	runtime.GOMAXPROCS(config.Agent.GoMaxProcForAgentWorker)

	// initialize logger
	log := logger.SSMLogger(true)
	defer log.Close()
	defer log.Flush()

	// Adjust OOM score to protect the SSM agent from being killed by the OOM killer
	platform.SetOOMScoreAdjust(log)

	// parse input parameters
	parseFlags(log)

	// run agent
	run(log, true)
}
