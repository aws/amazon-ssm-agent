package network

import (
	"net/http"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

func GetDefaultTransport(log log.T) *http.Transport {
	result := http.DefaultTransport.(*http.Transport).Clone()
	result.TLSClientConfig = GetDefaultTLSConfig(log)
	return result
}
