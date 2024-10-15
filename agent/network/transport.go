package network

import (
	"net/http"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

func GetDefaultTransport(log log.T, appConfig appconfig.SsmagentConfig) *http.Transport {
	result := http.DefaultTransport.(*http.Transport).Clone()
	result.TLSClientConfig = GetDefaultTLSConfig(log, appConfig)
	return result
}
