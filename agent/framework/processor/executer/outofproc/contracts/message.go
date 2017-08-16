package contracts

//TODO we need to move the DocumentResult model to this package,
import "github.com/aws/amazon-ssm-agent/agent/jsonutil"

type MessageType string

//Message types
const (
	MessageTypePluginConfig = "pluginconfig"
	MessageTypeComplete     = "complete"
	MessageTypeReply        = "reply"
	MessageTypeCancel       = "cancel"
)

var versions = []string{"1.0"}

//TODO Content should be interface{} to be marshalled based off the type
type Message struct {
	Version string      `json:"version"`
	Type    MessageType `json:"type"`
	Content string      `json:"content"`
}

type MessagingBackend interface {
	Accept() <-chan string
	//Process a given datagram, should not be blocked
	Process(string) error
	Close()
}

//GetLatestVersion retrieves the current latest message version of the agent build
func GetLatestVersion() string {
	return versions[len(versions)-1]
}

//CreateDatagram marshals a given arbitrary object to raw json string
//Message schema is determined by the current version, content struct is indicated by type field
//TODO add version handling
func CreateDatagram(t MessageType, content interface{}) (string, error) {
	contentStr, err := jsonutil.Marshal(content)
	if err != nil {
		return "", err
	}
	message := Message{
		Version: GetLatestVersion(),
		Type:    t,
		Content: contentStr,
	}
	datagram, err := jsonutil.Marshal(message)
	if err != nil {
		return "", err
	}
	return datagram, nil
}

//TODO add version and error handling
func ParseDatagram(datagram string) (MessageType, string) {
	message := Message{}
	jsonutil.Unmarshal(datagram, &message)
	return message.Type, message.Content
}
