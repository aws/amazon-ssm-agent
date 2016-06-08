// +build darwin freebsd linux netbsd openbsd

package main

import logger "github.com/aws/amazon-ssm-agent/agent/log"

func main() {
	// initialize logger
	log := logger.Logger()
	defer log.Close()
	defer log.Flush()

	// parse input parameters
	parseFlags(log)

	// run agent
	run(log)
}
