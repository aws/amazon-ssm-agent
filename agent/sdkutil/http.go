package sdkutil

import (
	"net/http"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/tlsconfig"
)

func GetDefaultTransport(log log.T) *http.Transport {
	result := http.DefaultTransport.(*http.Transport).Clone()
	result.TLSClientConfig = tlsconfig.GetDefaultTLSConfig(log)
	return result
}
