package identity

import (
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	identityinterface "github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
	"github.com/aws/aws-sdk-go/aws/credentials"
)

type agentIdentityCacher struct {
	instanceID         string
	shortInstanceID    string
	region             string
	availabilityZone   string
	availabilityZoneId string
	instanceType       string
	creds              *credentials.Credentials
	identityType       string
	mutex              sync.Mutex
	log                log.T
	client             identityinterface.IAgentIdentityInner
	endpointHelper     endpoint.IEndpointHelper
}

type createIdentityFunc func(log.T, *appconfig.SsmagentConfig) []identityinterface.IAgentIdentityInner

// allIdentityGenerators store all the available identity types and their generator functions. init inside identity definition add to
var allIdentityGenerators map[string]createIdentityFunc
