// +build windows

package main

import (
	"github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin/cloudwatch"
)

const serviceName = "AmazonSSMAgent"
const imageStateComplete = "IMAGE_STATE_COMPLETE"

func main() {
	// initialize logger
	log := ssmlog.SSMLogger(true)
	defer log.Close()
	defer log.Flush()

	// parse input parameters
	parseFlags(log)

	//Check if there's cloudwatch json config file, and skip hibernation check if configure CW is enabled
	shouldCheckHibernation := true
	err := cloudwatch.Instance().Update(log)
	if err == nil && cloudwatch.Instance().GetIsEnabled() {
		shouldCheckHibernation = false
	}

	run(log, shouldCheckHibernation)
}
