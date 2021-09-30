//go:build freebsd || linux || netbsd || openbsd || darwin
// +build freebsd linux netbsd openbsd darwin

package main

import (
	logger "github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
)

func main() {
	// parse input parameters
	parseFlags()
	handleAgentVersionFlag()

	// initialize logger
	log := logger.SSMLogger(true)
	defer log.Close()
	defer log.Flush()

	handleRegistrationAndFingerprintFlags(log)

	// run agent
	run(log)
}
