package dockercontainer

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	dockercontext "golang.org/x/net/context"
)

const (
	//Action values
	CREATE = "Create"
	START = "Start"
	RUN = "Run"
	STOP = "Stop"
	EXEC = "Exec"
	INSPECT = "Inspect"
	LOGS = "Logs"
	PS = "Ps"
	STATS = "Stats"
	PULL = "Pull"
	LIST = "List"
	RMI = "Rmi"

	// defaultExecutionTimeoutInSeconds represents default timeout time for execution of command in seconds
	defaultExecutionTimeoutInSeconds = 3600

)

var dockerExecCommand = "docker.exe"
var duration_Seconds time.Duration = 30 * time.Second

// Plugin is the type for the plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
	dockerClient *client.Client
}

// RunCommandPluginInput represents one set of commands executed by the RunCommand plugin.
type DockerContainerPluginInput struct {
	contracts.PluginInput
	Action           string
	ID               string
	WorkingDirectory string
	TimeoutSeconds   interface{}
	Container        string
	Cmd              []string
	Image            string
	Memory           string
	CpuShares        string
	Volume           []string
	env              string
	user             string
	ExposePort       string
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

	plugin.dockerClient, err = client.NewClient(client.DefaultDockerHost, client.DefaultVersion, nil, nil)
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
	log.Info("********************************starting container plugin**************************************")
	var outputBytes []byte
	switch pluginInput.Action {
	case CREATE:
		config := container.Config{Image: pluginInput.Image, Cmd: pluginInput.Cmd}

		hostConfig := container.HostConfig{}

		hostConfig.Binds = pluginInput.Volume

		networkingConfig := network.NetworkingConfig{}
		log.Info("Container create")
		response, err := p.dockerClient.ContainerCreate(dockercontext.Background(), &config, &hostConfig, &networkingConfig, pluginInput.Container)
		if err != nil {
			log.Error("Error Creating container", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
		log.Info("Container create passed", err)
		outputBytes, err = json.Marshal(response)
		if err != nil {
			log.Error("Error marshalling json output", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
	case START:
		err = p.dockerClient.ContainerStart(dockercontext.Background(), pluginInput.Container, types.ContainerStartOptions{})
		if err != nil {
			log.Error("Error starting container", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
	case RMI:
		err = p.dockerClient.ContainerRemove(dockercontext.Background(), pluginInput.Container, types.ContainerRemoveOptions{Force: true, RemoveVolumes: false, RemoveLinks: false})
		if err != nil {
			log.Error("Error removing container", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
	case STOP:
		err = p.dockerClient.ContainerStop(dockercontext.Background(), pluginInput.Container, &duration_Seconds)
		if err != nil {
			log.Error("Error stopping container", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
	case INSPECT:
		var containerJson types.ContainerJSON
		containerJson, err = p.dockerClient.ContainerInspect(dockercontext.Background(), pluginInput.Container)
		if err != nil {
			log.Error("Error inspecting container", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
		outputBytes, err = json.Marshal(containerJson)
		if err != nil {
			log.Error("Error marshalling json output", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
	case STATS:
		var containerStats types.ContainerStats
		containerStats, err = p.dockerClient.ContainerStats(dockercontext.Background(), pluginInput.Container, false)
		outputBytes, err = json.Marshal(containerStats)
		if err != nil {
			log.Error("Error marshalling json output", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
		defer containerStats.Body.Close()
	case LOGS:
		var output io.ReadCloser
		output, err = p.dockerClient.ContainerLogs(dockercontext.Background(), pluginInput.Container, types.ContainerLogsOptions{})
		if err != nil {
			log.Error("Error getting container logs", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
		defer output.Close()

		outputBytes, err = ioutil.ReadAll(output)
		if err != nil {
			log.Error("Error reading logs output", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}

	case PULL:
		var output io.ReadCloser
		output, err = p.dockerClient.ImagePull(dockercontext.Background(), pluginInput.Image, types.ImagePullOptions{})
		defer output.Close()
		outputBytes, err = ioutil.ReadAll(output)
		if err != nil {
			log.Error("Error reading pull output", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
	case PS:
		options := types.ContainerListOptions{All: true}
		var containers []types.Container
		containers, err = p.dockerClient.ContainerList(dockercontext.Background(), options)
		if err != nil {
			log.Info("ContainerList failed", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
		outputBytes, err = json.Marshal(containers)
		if err != nil {
			log.Error("Error marshalling json output", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
	default:
		out.MarkAsFailed(log, fmt.Errorf("Docker Action is set to unsupported value: %v", pluginInput.Action))
		return out
	}
	out.Stdout = string(outputBytes)
	out.ExitCode = 0
	out.Status = contracts.ResultStatusSuccess

	// Upload output to S3
	uploadOutputToS3BucketErrors := p.ExecuteUploadOutputToS3Bucket(log, pluginInput.ID, orchestrationDir, outputS3BucketName, outputS3KeyPrefix, useTempDirectory, tempDir, out.Stdout, out.Stderr)
	out.Errors = append(out.Errors, uploadOutputToS3BucketErrors...)

	// Return Json indented response
	responseContent, _ := jsonutil.Marshal(out)
	log.Debug("Returning response:\n", jsonutil.Indent(responseContent))
	return
}
