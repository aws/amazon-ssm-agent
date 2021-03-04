// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package manager encapsulates everything related to long running plugin manager that starts, stops & configures long running plugins
package manager

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/longrunning"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/datastore"
	managerContracts "github.com/aws/amazon-ssm-agent/agent/longrunning/plugin"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin/cloudwatch"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/carlescere/scheduler"
)

const (
	//name is the core module name for long running plugins manager
	Name = "LongRunningPluginsManager"

	// NameOfCloudWatchJsonFile is the name of ec2 config cloudwatch local configuration file
	NameOfCloudWatchJsonFile = "AWS.EC2.Windows.CloudWatch.json"

	//number of long running workers
	NumberOfLongRunningPluginWorkers = 5

	//number of cancel workers
	NumberOfCancelWorkers = 5

	//poll frequency for managing lifecycle of long running plugins
	PollFrequencyMinutes = 15

	//hardStopTimeout is the time before the manager will be shutdown during a hardstop = 4 seconds
	HardStopTimeout = 4 * time.Second

	//softStopTimeout is the time before the manager will be shutdown during a softstop = 20 seconds
	SoftStopTimeout = 20 * time.Second
)

// T manages long running plugins - get information of long running plugins and starts, stops & configures long running plugins
type T interface {
	contracts.ICoreModule
	GetRegisteredPlugins() map[string]managerContracts.Plugin
	StopPlugin(name string, cancelFlag task.CancelFlag) (err error)
	StartPlugin(name, configuration string, orchestrationDir string, cancelFlag task.CancelFlag, out iohandler.IOHandler) (err error)
	EnsurePluginRegistered(name string, plugin managerContracts.Plugin) (err error)
}

// Manager is the core module - that manages long running plugins
type Manager struct {
	context context.T

	dataStore dataStoreT

	//task pool to run long running plugins
	startPlugin task.Pool

	//task pool to stop long running plugins
	stopPlugin task.Pool

	//stores all writeable information about currently long running plugins
	runningPlugins map[string]managerContracts.PluginInfo

	//stores references of all the registered long running plugins
	registeredPlugins map[string]managerContracts.Plugin

	//manages lifecycle of all long running plugins
	managingLifeCycleJob *scheduler.Job

	//manages file system related functions
	fileSysUtil longrunning.FileSysUtil

	//ec2config's configuration xml parser
	ec2ConfigXmlParser cloudwatch.Ec2ConfigXmlParser
}

var singletonInstance *Manager
var once sync.Once

// EnsureManagerIsInitialized ensures that manager is initialized at least once
func EnsureInitialization(context context.T) {
	//todo: After we start using 1 task pool for entire agent (even for core modules), we can then move all initializations to init()

	//only components with access to context are expected to call this

	//this ensures that only one instance of lrpm exists
	once.Do(func() {
		managerContext := context.With("[" + Name + "]")
		log := managerContext.Log()
		//initialize pluginsInfo (which will store all information about long running plugins)
		plugins := map[string]managerContracts.PluginInfo{}
		//load all registered plugins
		regPlugins := RegisteredPlugins(context)
		jsonB, _ := json.Marshal(&regPlugins)
		log.Infof("registered plugins: %s", string(jsonB))

		// startPlugin and stopPlugin will be processed by separate worker pools
		// so we can define the number of workers for each pool
		cancelWaitDuration := 10000 * time.Millisecond
		clock := times.DefaultClock
		startPluginPool := task.NewPool(log, NumberOfLongRunningPluginWorkers, cancelWaitDuration, clock)
		stopPluginPool := task.NewPool(log, NumberOfCancelWorkers, cancelWaitDuration, clock)

		fileSysUtil := &longrunning.FileSysUtilImpl{}

		ec2ConfigXmlParser := &cloudwatch.Ec2ConfigXmlParserImpl{
			FileSysUtil: fileSysUtil,
		}

		dataStore := ds{
			dsImpl:  datastore.FsStore{},
			context: context,
		}

		singletonInstance = &Manager{
			context:            managerContext,
			dataStore:          dataStore,
			startPlugin:        startPluginPool,
			stopPlugin:         stopPluginPool,
			runningPlugins:     plugins,
			registeredPlugins:  regPlugins,
			fileSysUtil:        fileSysUtil,
			ec2ConfigXmlParser: ec2ConfigXmlParser,
		}
	})

}

// GetInstance returns an instance of Manager if its initialized otherwise it returns an error
func GetInstance() (*Manager, error) {
	lock.Lock()
	defer lock.Unlock()

	if singletonInstance == nil {
		return nil, errors.New("lrpm isn't initialized yet")
	} else {
		return singletonInstance, nil
	}
}

// GetRegisteredPlugins returns a map of all registered long running plugins
func (m *Manager) GetRegisteredPlugins() map[string]managerContracts.Plugin {
	return m.registeredPlugins
}

// Name returns the module name
func (m *Manager) ModuleName() string {
	return Name
}

// Execute starts long running plugin manager
func (m *Manager) ModuleExecute() (err error) {
	log := m.context.Log()
	defer func() {
		if msg := recover(); msg != nil {
			log.Errorf("long running manager ModuleExecute run panic: %v", msg)
		}
	}()
	log.Infof("starting long running plugin manager")
	//read from data store to determine if there were any previously long running plugins which need to be started again
	var dataStoreMap map[string]managerContracts.PluginInfo
	dataStoreMap, err = m.dataStore.Read()
	if len(dataStoreMap) != 0 {
		m.runningPlugins = dataStoreMap
	}

	if err != nil {
		log.Errorf("%s is exiting - unable to read from data store", m.ModuleName())
		return
	}

	//revive older long running plugins if they were running before
	if len(m.runningPlugins) > 0 {
		for pluginName, pluginInfo := range m.runningPlugins {
			//get the corresponding registered plugin
			p, exists := m.registeredPlugins[pluginName]
			if !exists {
				//remove previously running plugins with no registered handlers
				delete(m.runningPlugins, pluginName)
				continue
			}
			p.Info = pluginInfo
			if pluginName == appconfig.PluginNameCloudWatch {
				//skip CW plugin since it'll be handled later
				continue
			}
			log.Infof("Detected %s as a previously executing long running plugin. Starting that plugin again", p.Info.Name)
			//submit the work of long running plugin to the task pool
			/*
				Note: All long running plugins are singleton in nature - hence jobId = plugin name.
				This is in sync with our task-pool - which rejects jobs with duplicate jobIds.
			*/
			//todo: orchestrationDir should be set accordingly - 3rd parameter for Start
			shortInstanceID, _ := m.context.Identity().ShortInstanceID()
			orchestrationRootDir := filepath.Join(
				appconfig.DefaultDataStorePath,
				shortInstanceID,
				appconfig.DefaultDocumentRootDirName,
				m.context.AppConfig().Agent.OrchestrationRootDir)
			orchestrationDir := fileutil.BuildPath(orchestrationRootDir)

			ioConfig := contracts.IOConfiguration{
				OrchestrationDirectory: orchestrationDir,
				OutputS3BucketName:     "",
				OutputS3KeyPrefix:      "",
			}
			out := iohandler.NewDefaultIOHandler(m.context, ioConfig)
			defer out.Close()
			out.Init(p.Info.Name)
			p.Handler.Start(p.Info.Configuration, "", task.NewChanneledCancelFlag(), out)
			out.Close()
			m.registeredPlugins[pluginName] = p
		}
	} else {
		log.Infof("there aren't any long running plugin to execute")

	}

	//if no previous CW has been found, start a new one based on the json config
	if isPlatformSupported(log, appconfig.PluginNameCloudWatch) {
		m.configCloudWatch()
	}

	//schedule periodic health check of all long running plugins
	if m.managingLifeCycleJob, err = scheduler.Every(PollFrequencyMinutes).Minutes().Run(m.ensurePluginsAreRunning); err != nil {
		log.Errorf("unable to schedule long running plugins manager. %v", err)
	}

	return
}

// RequestStop handles the termination of the long running plugin manager
func (m *Manager) ModuleRequestStop(stopType contracts.StopType) (err error) {
	var waitTimeout time.Duration

	if stopType == contracts.StopTypeSoftStop {
		waitTimeout = SoftStopTimeout
	} else {
		waitTimeout = HardStopTimeout
	}

	var wg sync.WaitGroup

	// stop lifecycle management job that monitors execution of all long running plugins
	m.stopLifeCycleManagementJob()

	//there is no need to stop all individual plugins - because when the task pools are shutdown - all corresponding
	//jobs are also shutdown accordingly.

	// shutdown the send command pool in a separate go routine
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.context.Log().Errorf("Shutdown start plugin panic: %v", r)
				m.context.Log().Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()
		defer wg.Done()
		m.startPlugin.ShutdownAndWait(waitTimeout)
	}()

	// shutdown the cancel command pool in a separate go routine
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.context.Log().Errorf("Shutdown stop plugin panic: %v", r)
				m.context.Log().Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()
		defer wg.Done()
		m.stopPlugin.ShutdownAndWait(waitTimeout)
	}()

	if len(m.runningPlugins) > 0 {
		m.stopLongRunningPlugins(stopType)
	}

	// wait for everything to shutdown
	wg.Wait()
	return nil
}

// stopLongRunningPlugins requests the long running plugins to stop
func (m *Manager) stopLongRunningPlugins(stopType contracts.StopType) {
	log := m.context.Log()
	log.Infof("long running manager stop requested. Stop type: %v", stopType)

	var wg sync.WaitGroup
	i := 0
	for pluginName := range m.runningPlugins {
		go func(wgc *sync.WaitGroup, i int) {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("Stop long running plugins panic: %v", r)
					log.Errorf("Stacktrace:\n%s", debug.Stack())
				}
			}()
			if stopType == contracts.StopTypeSoftStop {
				wgc.Add(1)
				defer wgc.Done()
			}

			plugin := m.registeredPlugins[pluginName]
			if err := plugin.Handler.Stop(task.NewChanneledCancelFlag()); err != nil {
				log.Errorf("Plugin (%v) failed to stop with error: %v",
					pluginName,
					err)
			}

		}(&wg, i)
		i++
	}
}

// EnsurePluginRegistered adds a long-running plugin if it is not already in the registry
func (m *Manager) EnsurePluginRegistered(name string, plugin managerContracts.Plugin) (err error) {
	if _, exists := m.registeredPlugins[name]; !exists {
		m.registeredPlugins[name] = plugin
	}
	return nil
}

// configCloudWatch checks the local configuration file for cloud watch plugin to see if any updates to config
func (m *Manager) configCloudWatch() {
	log := m.context.Log()
	var err error

	var instanceId string
	if instanceId, err = m.context.Identity().InstanceID(); err != nil {
		log.Errorf("Cannot get instance id.")
		return
	}

	// Read from cloudwatch config file to check if any configuration need to make for cloud watch
	if err = cloudwatch.Instance().Update(log); err != nil {
		log.Debugf("There's no local configuration set for cloudwatch plugin. %v", err)

		// We also need to check if any configuration has been made by ec2 config before
		var hasConfiguration bool
		var localConfig bool
		if hasConfiguration, err = checkLegacyCloudWatchRunCommandConfig(log, instanceId, cloudwatch.Instance(), m.fileSysUtil); err != nil {
			log.Debugf("Have problem read configuration from ec2config file. %v", err)
			return
		}

		if !hasConfiguration {
			if localConfig, err = checkLegacyCloudWatchLocalConfig(log, cloudwatch.Instance(), m.ec2ConfigXmlParser, m.fileSysUtil); err != nil {
				log.Debugf("Have problem read configuration from ec2config file. %v", err)
				return
			}
		}

		if !hasConfiguration && !localConfig {
			log.Debug("There is no cloudwatch running in ec2 config service before.")
			return
		}
	}

	if cloudwatch.Instance().GetIsEnabled() {
		log.Infof("Detected cloud watch has updated configuration. Configuring that plugin again")
		// TODO need to check the folder
		orchestrationDir := fileutil.BuildPath(
			appconfig.DefaultDataStorePath,
			instanceId,
			appconfig.DefaultDocumentRootDirName)
		var config string
		if config, err = cloudwatch.Instance().ParseEngineConfiguration(); err != nil {
			log.Debug("Cannot parse EngineConfiguration to string format")
		}

		ioConfig := contracts.IOConfiguration{
			OrchestrationDirectory: orchestrationDir,
			OutputS3BucketName:     "",
			OutputS3KeyPrefix:      "",
		}
		out := iohandler.NewDefaultIOHandler(m.context, ioConfig)
		defer out.Close()
		out.Init(appconfig.PluginNameCloudWatch)
		if err = m.StartPlugin(
			appconfig.PluginNameCloudWatch,
			config,
			orchestrationDir,
			task.NewChanneledCancelFlag(), out); err != nil {
			log.Errorf("Failed to start the cloud watch plugin bacause: %s", err)
		}
		out.Close()

		// check if configure cloudwatch successfully
		stderrFilePath := fileutil.BuildPath(orchestrationDir, appconfig.PluginNameCloudWatch, "stderr")
		var errData []byte
		var errorReadingFile error
		if errData, errorReadingFile = m.fileSysUtil.ReadFile(stderrFilePath); errorReadingFile != nil {
			log.Errorf("Unable to read the stderr file - %s: %s", stderrFilePath, errorReadingFile.Error())
		}
		serr := string(errData)

		if len(serr) > 0 {
			log.Errorf("Unable to start the plugin - %s: %s", appconfig.PluginNameCloudWatch, serr)
			// Stop the plugin if configuration failed.
			if err := m.StopPlugin(appconfig.PluginNameCloudWatch, task.NewChanneledCancelFlag()); err != nil {
				log.Errorf("Unable to start the plugin - %s: %s", appconfig.PluginNameCloudWatch, err.Error())
			}
		}

	} else {
		log.Infof("Detected cloud watch has been requested to stop. Stoping the plugin")
		if err = m.StopPlugin(appconfig.PluginNameCloudWatch, task.NewChanneledCancelFlag()); err != nil {
			log.Errorf("Failed to stop the cloud watch plugin bacause: %s", err)
		}
	}
}

// checkLegacyCloudWatchRunCommandConfig checks if ec2config has cloudwatch configuration document running before
func checkLegacyCloudWatchRunCommandConfig(logger log.T, instanceId string, cwcInstance cloudwatch.CloudWatchConfig, fileSysUtil longrunning.FileSysUtil) (hasConfiguration bool, err error) {
	var engineConfigurationParser cloudwatch.EngineConfigurationParser
	var documentModel contracts.DocumentContent
	var content []byte

	storeFileName := fileutil.BuildPath(
		appconfig.EC2ConfigDataStorePath,
		instanceId,
		appconfig.ConfigurationRootDirName,
		appconfig.WorkersRootDirName,
		"aws.cloudWatch.ec2config")
	hasConfiguration = false

	if !fileSysUtil.Exists(storeFileName) {
		return
	}

	lock.RLock()
	defer lock.RUnlock()
	content, err = fileSysUtil.ReadFile(storeFileName)
	if err != nil {
		return
	}

	if json.Unmarshal(content, &documentModel); err != nil {
		return
	}
	logger.Debugf("unmarshal document model: %v", documentModel)

	pluginConfig := documentModel.RuntimeConfig[appconfig.PluginNameCloudWatch]
	if pluginConfig == nil || pluginConfig.Properties == nil {
		err = fmt.Errorf("%v doesn't contain %v", storeFileName, appconfig.PluginNameCloudWatch)
		return
	}

	// The legacy Ec2Config's plugin config properties may contain escaped characters.
	// Unmarshalling the raw string should correct the format to a tree of maps.
	rawIn := json.RawMessage(pluginConfig.Properties.(string))
	if err = json.Unmarshal([]byte(rawIn), &engineConfigurationParser); err != nil {
		return
	}

	logger.Debugf("unmarshal engine configuration - run command: %v", engineConfigurationParser)
	if err = cwcInstance.Enable(engineConfigurationParser.EngineConfiguration); err != nil {
		return
	}
	hasConfiguration = true

	return
}

// checkLegacyCloudWatchLocalConfig checks if users have cloudwatch local configuration before.
func checkLegacyCloudWatchLocalConfig(logger log.T, cwcInstance cloudwatch.CloudWatchConfig, ec2ConfigXmlParser cloudwatch.Ec2ConfigXmlParser, fileSysUtil longrunning.FileSysUtil) (hasConfiguration bool, err error) {
	var engineConfigurationParser cloudwatch.EngineConfigurationParser
	var content []byte
	var isEnabled bool
	hasConfiguration = false

	// first check the config.xml file to see if the cloudwatch plugin is enabled
	isEnabled, err = ec2ConfigXmlParser.IsCloudWatchEnabled()
	if err != nil || !isEnabled {
		return
	}

	configFileName := fileutil.BuildPath(
		appconfig.EC2ConfigSettingPath,
		NameOfCloudWatchJsonFile)

	if !fileSysUtil.Exists(configFileName) {
		return
	}

	lock.RLock()
	defer lock.RUnlock()
	content, err = fileSysUtil.ReadFile(configFileName)
	if err != nil {
		return
	}

	validContent := checkAndRemoveBomCharacters(content)

	// Update the config file with new configuration
	if err = json.Unmarshal(validContent, &engineConfigurationParser); err != nil {
		return
	}
	logger.Debugf("unmarshal engine configuration - cloud watch: %v", engineConfigurationParser)
	if err = cwcInstance.Enable(engineConfigurationParser.EngineConfiguration); err != nil {
		return
	}

	return true, nil
}

// checkAndRemoveBomCharacters checks if there is any invalid bom characters at the beginning of the bytes.
// If found, remove them to avoid unmarshall problem.
func checkAndRemoveBomCharacters(content []byte) []byte {
	bom := []byte{0xef, 0xbb, 0xbf} // UTF-8
	// if byte-order mark found
	if bytes.Equal(content[:3], bom) {
		content = content[3:]
	}
	return content
}
