package contracts

//TODO we need to move the DocumentResult model to this package,
import (
	"errors"

	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
)

type MessageType string

//Message types
const (
	MessageTypeStart   = "start"
	MessageTypeClose   = "close"
	MessageTypePayload = "payload"
	MessageTypeControl = "control"
)

var versions = []string{"1.0"}

//TODO Content should be interface{} to be marshalled based off the type
type Message struct {
	Version string `json:"version"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

//These 2 type of messages is heavily rely on the version control mechanism

func GetLastestVersion() string {
	return versions[len(versions)-1]
}

func Marshal(version, t string, obj interface{}) (msg Message, err error) {
	content, err := jsonutil.Marshal(obj)
	if err != nil {
		return
	}
	msg.Version = version
	msg.Type = t
	msg.Content = content
	return
}

func UnMarshal(msg Message, result interface{}) (err error) {
	v := msg.Version
	//The switch cases here, once checked in, should never be changed for backward compatibility
	switch v {
	case "1.0":
		err = jsonutil.Unmarshal(msg.Content, result)
		return
	default:
		err = errors.New("unsupported version")
		return
	}
}
