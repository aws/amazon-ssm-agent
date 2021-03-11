package manager

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

var lrpName = appconfig.PluginNameCloudWatch

func CreateResult(msg string, status contracts.ResultStatus, res *contracts.PluginResult) {
	res.Output = msg

	if status == contracts.ResultStatusFailed {
		res.StandardError = msg
		res.Code = 1
	} else {
		res.StandardOutput = msg
		res.Code = 0
	}

	res.Status = status
	return
}

func Invoke(context context.T, pluginID string, res *contracts.PluginResult, orchestrationDir string) {
	var lrpm T
	var err error
	var startType = res.StandardOutput
	var property string
	jsonutil.Remarshal(res.Output, &property)
	res.StandardOutput = ""
	res.Output = ""
	log := context.Log()
	lrpm, err = GetInstance()
	var pluginsMap = lrpm.GetRegisteredPlugins()
	if _, ok := pluginsMap[lrpName]; !ok {
		log.Errorf("Given plugin - %s is not registered", lrpName)
		CreateResult(fmt.Sprintf("Plugin %s is not registered by agent", lrpName),
			contracts.ResultStatusFailed, res)

		return
	}
	cancelFlag := task.NewChanneledCancelFlag()
	//NOTE: All long running plugins have json node similar to aws:cloudWatch as mentioned in SSM document - AWS-ConfigureCloudWatch

	//check if plugin is enabled or not - which would be stored in settings
	switch startType {
	case "Enabled":
		enablePlugin(context, orchestrationDir, pluginID, lrpm, cancelFlag, property, res)

	case "Disabled":
		log.Infof("Disabling %s", lrpName)
		if err = lrpm.StopPlugin(lrpName, cancelFlag); err != nil {
			log.Errorf("Unable to stop the plugin - %s: %s", pluginID, err.Error())
			CreateResult(fmt.Sprintf("Encountered error while stopping the plugin: %s", err.Error()),
				contracts.ResultStatusFailed, res)

		} else {
			CreateResult(fmt.Sprintf("Disabled the plugin - %s successfully", lrpName),
				contracts.ResultStatusSuccess, res)
			res.Status = contracts.ResultStatusSuccess
		}

	default:
		log.Errorf("Allowed Values of StartType: Enabled | Disabled but provided value is: %s", startType)
		CreateResult("Allowed Values of StartType: Enabled | Disabled",
			contracts.ResultStatusFailed, res)
	}

	return
}

func enablePlugin(context context.T, orchestrationDirectory string, pluginID string, lrpm T, cancelFlag task.CancelFlag, property string, res *contracts.PluginResult) {
	log := context.Log()
	log.Infof("Enabling %s", lrpName)

	//loading properties as string since aws:cloudWatch uses properties as string. Properties has new configuration for cloudwatch plugin.
	//For more details refer to AWS-ConfigureCloudWatch
	// TODO cannot check if string is a valid json for cloudwatch
	//stop the plugin before reconfiguring it
	log.Debugf("Stopping %s - before applying new configuration", lrpName)
	if err := lrpm.StopPlugin(lrpName, cancelFlag); err != nil {
		log.Errorf("Unable to stop the plugin - %s: %s", lrpName, err.Error())
	}

	ioConfig := contracts.IOConfiguration{
		OrchestrationDirectory: orchestrationDirectory,
		OutputS3BucketName:     res.OutputS3BucketName,
		OutputS3KeyPrefix:      res.OutputS3KeyPrefix,
	}
	out := iohandler.NewDefaultIOHandler(context, ioConfig)
	defer out.Close()
	out.Init(appconfig.PluginNameCloudWatch)

	//start the plugin with the new configuration
	if err := lrpm.StartPlugin(lrpName, property, orchestrationDirectory, cancelFlag, out); err != nil {
		log.Errorf("Unable to start the plugin - %s: %s", lrpName, err.Error())
		CreateResult(fmt.Sprintf("Encountered error while starting the plugin: %s", err.Error()),
			contracts.ResultStatusFailed, res)
	} else {

		if len(out.GetStderr()) > 0 {
			log.Errorf("Unable to start the plugin - %s: %s", lrpName, out.GetStderr())

			// Stop the plugin if configuration failed.
			if err := lrpm.StopPlugin(lrpName, cancelFlag); err != nil {
				log.Errorf("Unable to start the plugin - %s: %s", lrpName, err.Error())
			}

			CreateResult(fmt.Sprintf("Encountered error while starting the plugin: %s", out.GetStderr()),
				contracts.ResultStatusFailed, res)

		} else {
			log.Info("Start Cloud Watch successfully.")
			CreateResult("success", contracts.ResultStatusSuccess, res)
		}
	}
	out.Close()
	return
}
