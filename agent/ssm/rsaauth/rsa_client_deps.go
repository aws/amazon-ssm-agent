package rsaauth

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authtokenrequest"
	"github.com/aws/amazon-ssm-agent/agent/ssm/util"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

type IRsaClientDeps interface {
	NewStaticCredentials(id string, secret string, token string) *credentials.Credentials
	NewSession(config *aws.Config) (*session.Session, error)
	AwsConfig(log log.T, appConfig appconfig.SsmagentConfig, service string, region string) *aws.Config
	NewSsmSdk(p client.ConfigProvider, cfgs ...*aws.Config) *ssm.SSM
	NewAuthTokenClient(sdk *ssm.SSM) authtokenrequest.IClient
	MakeAddToUserAgentHandler(name string, version string, extra ...string) func(*request.Request)
	NewCredentials(provider credentials.Provider) *credentials.Credentials
}

type rsaClientDeps struct{}

var deps IRsaClientDeps = &rsaClientDeps{}

func (r *rsaClientDeps) NewStaticCredentials(id string, secret string, token string) *credentials.Credentials {
	return credentials.NewStaticCredentials(id, secret, token)
}

func (r *rsaClientDeps) NewSession(config *aws.Config) (*session.Session, error) {
	return session.NewSession(config)
}

func (r *rsaClientDeps) AwsConfig(log log.T, appConfig appconfig.SsmagentConfig, service string, region string) *aws.Config {
	return util.AwsConfig(log, appConfig, service, region)
}

func (r *rsaClientDeps) NewSsmSdk(p client.ConfigProvider, cfgs ...*aws.Config) *ssm.SSM {
	return ssm.New(p, cfgs...)
}

func (r *rsaClientDeps) NewAuthTokenClient(sdk *ssm.SSM) authtokenrequest.IClient {
	return authtokenrequest.NewClient(sdk)
}

func (r *rsaClientDeps) MakeAddToUserAgentHandler(name string, version string, extra ...string) func(*request.Request) {
	return request.MakeAddToUserAgentHandler(name, version, extra...)
}

func (r *rsaClientDeps) NewCredentials(provider credentials.Provider) *credentials.Credentials {
	return credentials.NewCredentials(provider)
}
