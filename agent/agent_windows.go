// +build windows

package main

import (
	"os"

	"github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin/cloudwatch"
	"github.com/aws/amazon-ssm-agent/agent/proxyconfig"
)

const serviceName = "AmazonSSMAgent"
const imageStateComplete = "IMAGE_STATE_COMPLETE"

func main() {
	// initialize logger
	log := ssmlog.SSMLogger(true)
	defer log.Close()
	defer log.Flush()

	proxyconfig.SetProxySettings(log)

	log.Infof("Proxy environment variables:")
	for _, name := range []string{"http_proxy", "https_proxy", "no_proxy"} {
		log.Infof(name + ": " + os.Getenv(name))
	}

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
