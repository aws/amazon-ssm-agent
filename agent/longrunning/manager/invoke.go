package manager

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
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

func Invoke(log logger.T, pluginID string, res *contracts.PluginResult, orchestrationDir string) {
	var lrpm T
	var err error
	var startType = res.StandardOutput
	var property string
	jsonutil.Remarshal(res.Output, &property)
	res.StandardOutput = ""
	res.Output = ""
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
		enablePlugin(log, orchestrationDir, pluginID, lrpm, cancelFlag, property, res)

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
		log.Errorf("Allowed Values of StartType: Enabled | Disabled")
		CreateResult("Allowed Values of StartType: Enabled | Disabled",
			contracts.ResultStatusFailed, res)
	}

	return
}

func enablePlugin(log logger.T, orchestrationDirectory string, pluginID string, lrpm T, cancelFlag task.CancelFlag, property string, res *contracts.PluginResult) {
	log.Infof("Enabling %s", lrpName)

	//loading properties as string since aws:cloudWatch uses properties as string. Properties has new configuration for cloudwatch plugin.
	//For more details refer to AWS-ConfigureCloudWatch
	// TODO cannot check if string is a valid json for cloudwatch
	//stop the plugin before reconfiguring it
	log.Debugf("Stopping %s - before applying new configuration", lrpName)
	if err := lrpm.StopPlugin(lrpName, cancelFlag); err != nil {
		log.Errorf("Unable to stop the plugin - %s: %s", lrpName, err.Error())
		CreateResult(fmt.Sprintf("Encountered error while stopping the plugin: %s", err.Error()),
			contracts.ResultStatusFailed, res)
		return
	}
	outputPath := fileutil.BuildPath(orchestrationDirectory, appconfig.PluginNameCloudWatch)
	stdoutFilePath := filepath.Join(outputPath, "stdout")
	stderrFilePath := filepath.Join(outputPath, "stderr")

	//start the plugin with the new configuration
	if err := lrpm.StartPlugin(lrpName, property, orchestrationDirectory, cancelFlag); err != nil {
		log.Errorf("Unable to start the plugin - %s: %s", lrpName, err.Error())
		CreateResult(fmt.Sprintf("Encountered error while starting the plugin: %s", err.Error()),
			contracts.ResultStatusFailed, res)
	} else {
		var errData []byte
		var errorReadingFile error
		if errData, errorReadingFile = ioutil.ReadFile(stderrFilePath); errorReadingFile != nil {
			log.Errorf("Unable to read the stderr file - %s: %s", stderrFilePath, errorReadingFile.Error())
		}
		serr := string(errData)

		if len(serr) > 0 {
			log.Errorf("Unable to start the plugin - %s: %s", lrpName, serr)

			// Stop the plugin if configuration failed.
			if err := lrpm.StopPlugin(lrpName, cancelFlag); err != nil {
				log.Errorf("Unable to start the plugin - %s: %s", lrpName, err.Error())
			}

			CreateResult(fmt.Sprintf("Encountered error while starting the plugin: %s", serr),
				contracts.ResultStatusFailed, res)

		} else {
			log.Info("Start Cloud Watch successfully.")
			CreateResult("success", contracts.ResultStatusSuccess, res)
		}
	}
	defaultPlugin := pluginutil.DefaultPlugin{}
	// Upload output to S3
	uploadOutputToS3BucketErrors := defaultPlugin.UploadOutputToS3Bucket(log, pluginID, outputPath, res.OutputS3BucketName, res.OutputS3KeyPrefix, false, "", stdoutFilePath, stderrFilePath)
	if len(uploadOutputToS3BucketErrors) > 0 {
		log.Errorf("Unable to upload the logs - %s: %s", pluginID, uploadOutputToS3BucketErrors)
	}
	return
}
