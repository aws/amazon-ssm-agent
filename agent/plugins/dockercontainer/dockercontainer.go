package dockercontainer

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	//Action values
	CREATE  = "Create"
	START   = "Start"
	RUN     = "Run"
	STOP    = "Stop"
	RM      = "Rm"
	EXEC    = "Exec"
	INSPECT = "Inspect"
	LOGS    = "Logs"
	PS      = "Ps"
	STATS   = "Stats"
	PULL    = "Pull"
	IMAGES  = "Images"
	RMI     = "Rmi"

	// defaultExecutionTimeoutInSeconds represents default timeout time for execution of command in seconds
	defaultExecutionTimeoutInSeconds = 3600
)

var dockerExecCommand = "docker.exe"
var duration_Seconds time.Duration = 30 * time.Second

// Plugin is the type for the plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
}

// RunCommandPluginInput represents one set of commands executed by the RunCommand plugin.
type DockerContainerPluginInput struct {
	contracts.PluginInput
	Action           string
	ID               string
	WorkingDirectory string
	TimeoutSeconds   interface{}
	Container        string
	Cmd              string
	Image            string
	Memory           string
	CpuShares        string
	Volume           []string
	Env              string
	User             string
	Publish          string
}

// PSModulePluginOutput represents the output of the plugin
type DockerContainerPluginOutput struct {
	contracts.PluginOutput
}

// Failed marks plugin as Failed
func (out *DockerContainerPluginOutput) MarkAsFailed(log log.T, err error) {
	out.ExitCode = 1
	out.Status = contracts.ResultStatusFailed
	if len(out.Stderr) != 0 {
		out.Stderr = fmt.Sprintf("\n%v\n%v", out.Stderr, err.Error())
	} else {
		out.Stderr = fmt.Sprintf("\n%v", err.Error())
	}
	log.Error(err.Error())
	out.Errors = append(out.Errors, err.Error())
}

// NewPlugin returns a new instance of the plugin.
func NewPlugin(pluginConfig pluginutil.PluginConfig) (*Plugin, error) {
	var plugin Plugin
	var err error
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	plugin.Uploader = pluginutil.GetS3Config()
	plugin.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(plugin.UploadOutputToS3Bucket)

	exec := executers.ShellCommandExecuter{}
	plugin.ExecuteCommand = pluginutil.CommandExecuter(exec.Execute)

	return &plugin, err
}

// Name returns the name of the plugin
func Name() string {
	return appconfig.PluginNameDockerContainer
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of DockerContainerPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, pluginRunner runpluginutil.PluginRunner) (res contracts.PluginResult) {
	log := context.Log()
	log.Infof("%v started with configuration %v", Name(), config)
	res.StartDateTime = time.Now()
	defer func() {
		res.EndDateTime = time.Now()
	}()

	//loading Properties as list since aws:psModule uses properties as list
	var properties []interface{}
	if properties, res = pluginutil.LoadParametersAsList(log, config.Properties); res.Code != 0 {

		pluginutil.PersistPluginInformationToCurrent(log, Name(), config, res)
		return res
	}

	out := make([]DockerContainerPluginOutput, len(properties))
	for i, prop := range properties {
		// check if a reboot has been requested
		if rebooter.RebootRequested() {
			log.Info("A plugin has requested a reboot.")
			return
		}

		if cancelFlag.ShutDown() {
			out[i].ExitCode = 1
			out[i].Status = contracts.ResultStatusFailed
			break
		}

		if cancelFlag.Canceled() {
			out[i].ExitCode = 1
			out[i].Status = contracts.ResultStatusCancelled
			break
		}

		out[i] = p.runCommandsRawInput(log, prop, config.OrchestrationDirectory, cancelFlag, config.OutputS3BucketName, config.OutputS3KeyPrefix)
	}

	if len(properties) > 0 {
		res.Code = out[0].ExitCode
		res.Status = out[0].Status
		res.Output = out[0].String()
	}

	pluginutil.PersistPluginInformationToCurrent(log, Name(), config, res)

	return res
}

// runCommandsRawInput executes one set of commands and returns their output.
// The input is in the default json unmarshal format (e.g. map[string]interface{}).
func (p *Plugin) runCommandsRawInput(log log.T, rawPluginInput interface{}, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out DockerContainerPluginOutput) {
	var pluginInput DockerContainerPluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	log.Debugf("Plugin input %v", pluginInput)
	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		out.MarkAsFailed(log, errorString)
		return
	}

	return p.runCommands(log, pluginInput, orchestrationDirectory, cancelFlag, outputS3BucketName, outputS3KeyPrefix)
}

// runCommands executes one set of commands and returns their output.
func (p *Plugin) runCommands(log log.T, pluginInput DockerContainerPluginInput, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out DockerContainerPluginOutput) {
	var err error

	// if no orchestration directory specified, create temp directory
	var useTempDirectory = (orchestrationDirectory == "")
	var tempDir string
	if useTempDirectory {
		if tempDir, err = ioutil.TempDir("", "Ec2RunCommand"); err != nil {
			out.Errors = append(out.Errors, err.Error())
			log.Error(err)
			return
		}
		orchestrationDirectory = tempDir
	}

	orchestrationDir := fileutil.BuildPath(orchestrationDirectory, pluginInput.ID)
	log.Debugf("OrchestrationDir %v ", orchestrationDir)

	// create orchestration dir if needed
	if err = fileutil.MakeDirs(orchestrationDir); err != nil {
		log.Debug("failed to create orchestrationDir directory", orchestrationDir, err)
		out.Errors = append(out.Errors, err.Error())
		return
	}
	var command string = "docker"
	var parameters []string
	switch pluginInput.Action {
	case CREATE, RUN:
		if len(pluginInput.Image) == 0 {
			log.Errorf("Action %s requires paramter image", pluginInput.Action, err)
			out.Errors = append(out.Errors, err.Error())
			return out
		}
		parameters = make([]string, 0)
		if pluginInput.Action == RUN {
			parameters = append(parameters, "run", "-d")
		} else {
			parameters = append(parameters, "create")
		}
		if len(pluginInput.Volume) > 0 && len(pluginInput.Volume[0]) > 0 {
			out.Stdout += "pluginInput.Volume:" + strconv.Itoa(len(pluginInput.Volume))

			log.Info("pluginInput.Volume", len(pluginInput.Volume))
			parameters = append(parameters, "--volume")
			for _, vol := range pluginInput.Volume {
				log.Info("pluginInput.Volume item", vol)
				parameters = append(parameters, vol)
			}
		}
		if len(pluginInput.Container) > 0 {
			parameters = append(parameters, "--name")
			parameters = append(parameters, pluginInput.Container)
		}
		if len(pluginInput.Memory) > 0 {
			parameters = append(parameters, "--memory")
			parameters = append(parameters, pluginInput.Memory)
		}
		if len(pluginInput.CpuShares) > 0 {
			parameters = append(parameters, "--cpu-shares")
			parameters = append(parameters, pluginInput.CpuShares)
		}
		if len(pluginInput.Publish) > 0 {
			parameters = append(parameters, "--publish")
			parameters = append(parameters, pluginInput.Publish)
		}
		if len(pluginInput.Env) > 0 {
			parameters = append(parameters, "--env")
			parameters = append(parameters, pluginInput.Env)
		}
		if len(pluginInput.User) > 0 {
			parameters = append(parameters, "--user")
			parameters = append(parameters, pluginInput.User)
		}
		parameters = append(parameters, pluginInput.Image)
		parameters = append(parameters, pluginInput.Cmd)

	case START:
		parameters = append(parameters, "start")
		if len(pluginInput.Container) == 0 {
			log.Error("Action Start requires paramter container", err)
			out.Errors = append(out.Errors, err.Error())
			return out
		}
		parameters = append(parameters, pluginInput.Container)

	case RM:
		parameters = append(parameters, "rm")
		if len(pluginInput.Container) == 0 {
			log.Error("Action Rm requires paramter container", err)
			out.Errors = append(out.Errors, err.Error())
			return out
		}
		parameters = append(parameters, pluginInput.Container)

	case STOP:
		parameters = append(parameters, "stop")
		if len(pluginInput.Container) == 0 {
			log.Error("Action Stop requires paramter container", err)
			out.Errors = append(out.Errors, err.Error())
			return out
		}
		parameters = append(parameters, pluginInput.Container)

	case EXEC:
		parameters = append(parameters, "exec")
		if len(pluginInput.Container) == 0 {
			log.Error("Action Exec requires paramter container", err)
			out.Errors = append(out.Errors, err.Error())
			return out
		}
		if len(pluginInput.Cmd) == 0 {
			log.Error("Action Exec requires paramter Cmd", err)
			out.Errors = append(out.Errors, err.Error())
			return out
		}
		if len(pluginInput.User) > 0 {
			parameters = append(parameters, "--user")
			parameters = append(parameters, pluginInput.User)
		}
		parameters = append(parameters, pluginInput.Container)
		parameters = append(parameters, pluginInput.Cmd)
	case INSPECT:
		parameters = append(parameters, "inspect")
		if len(pluginInput.Container) == 0 || len(pluginInput.Image) == 0 {
			log.Error("Action Inspect requires paramter container or image", err)
			out.Errors = append(out.Errors, err.Error())
			return out
		}
		parameters = append(parameters, pluginInput.Container)
		parameters = append(parameters, pluginInput.Image)
	case STATS:
		parameters = append(parameters, "stats")
		parameters = append(parameters, "--no-stream")
		if len(pluginInput.Container) > 0 {
			parameters = append(parameters, pluginInput.Container)
		}
	case LOGS:
		parameters = append(parameters, "logs")
		if len(pluginInput.Container) == 0 {
			log.Error("Action Rm requires paramter container", err)
			out.Errors = append(out.Errors, err.Error())
			return out
		}
		parameters = append(parameters, pluginInput.Container)
	case PULL:
		parameters = append(parameters, "pull")
		if len(pluginInput.Image) == 0 {
			log.Error("Action Pull requires paramter image", err)
			out.Errors = append(out.Errors, err.Error())
			return out
		}
		parameters = append(parameters, pluginInput.Image)
	case IMAGES:
		parameters = append(parameters, "images")
	case RMI:
		parameters = append(parameters, "rmi")
		if len(pluginInput.Image) == 0 {
			log.Error("Action Rmi requires paramter image", err)
			out.Errors = append(out.Errors, err.Error())
			return out
		}
		parameters = append(parameters, pluginInput.Image)

	case PS:
		parameters = append(parameters, "ps", "--all")
	default:
		out.MarkAsFailed(log, fmt.Errorf("Docker Action is set to unsupported value: %v", pluginInput.Action))
		return out
	}

	out.Stdout += command + " "
	for _, parameter := range parameters {
		out.Stdout += parameter + " "
	}
	out.Stdout += "\n"
	log.Info(out.Stdout)
	var output string
	output, err = dep.UpdateUtilExeCommandOutput(1800, log, command, parameters, "", "", "", "", true)
	if err != nil {
		log.Error("Error running docker command ", err)
		out.Errors = append(out.Errors, err.Error())
		return out
	}
	log.Info("Save-Module output:", output)
	out.Stdout += output

	out.ExitCode = 0
	out.Status = contracts.ResultStatusSuccess
	// Upload output to S3
	uploadOutputToS3BucketErrors := p.ExecuteUploadOutputToS3Bucket(log, pluginInput.ID, orchestrationDir, outputS3BucketName, outputS3KeyPrefix, useTempDirectory, tempDir, out.Stdout, out.Stderr)
	out.Errors = append(out.Errors, uploadOutputToS3BucketErrors...)

	// Return Json indented response
	responseContent, _ := jsonutil.Marshal(out)
	log.Debug("Returning response:\n", jsonutil.Indent(responseContent))
	return out
}
