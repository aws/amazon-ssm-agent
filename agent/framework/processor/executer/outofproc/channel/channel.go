package channel

import (
	"path"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

const (
	ModeMaster Mode = "master"
	ModeWorker Mode = "worker"
)
const (
	defaultChannelBufferSize = 100
	defaultFileChannelPath   = "channels"
)

type Mode string

//Channel is an interface for raw json datagram transmission, it is designed to adopt both file ad named pipe
type Channel interface {
	//send a raw json datagram to the channel, should be non-blocked
	Send(string) error
	GetMessageChannel() chan string
	Close()
}

//find the folder named as "documentID" under the default root dir
//if not found, create a new filechannel under the default root dir
//return the channel and the found flag
func CreateFileChannel(log log.T, mode Mode, filename string) (Channel, error, bool) {
	instanceID, err := platform.InstanceID()
	if err != nil {
		log.Errorf("failed to load instance ID: %v", err)
		return nil, err, false
	}
	list, err := fileutil.ReadDir(path.Join(appconfig.DefaultDataStorePath, instanceID, defaultFileChannelPath))
	if err != nil {
		log.Errorf("failed to read the default channel root directory: %v, creating a new Channel", err)
		f, err := NewFileWatcherChannel(log, mode, path.Join(appconfig.DefaultDataStorePath, instanceID, defaultFileChannelPath, filename))
		return f, err, false
	}
	for _, val := range list {
		if val.Name() == filename {
			log.Errorf("channel: %v found", filename)
			f, err := NewFileWatcherChannel(log, ModeMaster, path.Join(appconfig.DefaultDataStorePath, instanceID, defaultFileChannelPath, filename))
			return f, err, true
		}
	}

	f, err := NewFileWatcherChannel(log, mode, path.Join(appconfig.DefaultDataStorePath, instanceID, defaultFileChannelPath, filename))
	return f, err, false
}
