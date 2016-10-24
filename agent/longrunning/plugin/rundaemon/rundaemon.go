package rundaemon

import (
	"os"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// Plugin is the type for the configureDaemon plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
	Process     os.Process
	ExeLocation string // this is the version directory for a particular daemon
	Name        string // name of the daemon
	CommandLine string // command line to launch the daemon (with the exelocation as working directory)
}

func (p *Plugin) IsRunning(context context.T) bool {
	log := context.Log()
	log.Infof("IsRunning check for daemon %v", p.Name)
	return false // TODO:DAEMON check to see if process is alive (false for now to force regular restarts and see the logs
}

func (p *Plugin) Start(context context.T, configuration string, orchestrationDir string, cancelFlag task.CancelFlag) error {
	log := context.Log()
	log.Infof("Starting %v /nCommand: %v /nConfig: %v", p.Name, p.CommandLine, configuration)
	return nil // TODO:DAEMON spawn process
}

func (p *Plugin) Stop(context context.T, cancelFlag task.CancelFlag) error {
	log := context.Log()
	log.Infof("Stopping %v", p.Name)
	return nil // TODO:DAEMON end process
}
