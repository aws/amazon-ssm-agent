package rsaauth

import (
	"context"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authtokenrequest"
	"github.com/aws/amazon-ssm-agent/agent/ssm/util"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type IRsaClientDeps interface {
	NewStaticCredentials(id string, secret string, token string) aws.Credentials
	NewStaticCredentialsProvider(id string, secret string, token string) credentials.StaticCredentialsProvider
	AwsConfig(log log.T, appConfig appconfig.SsmagentConfig, service string, region string) aws.Config
	NewSsmSdk(cfgs aws.Config) *ssm.Client
	NewAuthTokenClient(sdk *ssm.Client) authtokenrequest.IClient
	NewCredentials(provider credentials.StaticCredentialsProvider) aws.Credentials
}

type rsaClientDeps struct{}

var deps IRsaClientDeps = &rsaClientDeps{}

func (r *rsaClientDeps) NewStaticCredentials(id string, secret string, token string) aws.Credentials {
	return aws.Credentials{AccessKeyID: id,
		SecretAccessKey: secret, SessionToken: token}
}

func (r *rsaClientDeps) NewStaticCredentialsProvider(id string, secret string, token string) credentials.StaticCredentialsProvider {
	return credentials.StaticCredentialsProvider{Value: r.NewStaticCredentials(id, secret, token)}
}

func (r *rsaClientDeps) AwsConfig(log log.T, appConfig appconfig.SsmagentConfig, service string, region string) aws.Config {
	return util.AwsConfig(log, appConfig, service, region)
}

func (r *rsaClientDeps) NewSsmSdk(cfgs aws.Config) *ssm.Client {
	return ssm.NewFromConfig(cfgs)
}

func (r *rsaClientDeps) NewAuthTokenClient(sdk *ssm.Client) authtokenrequest.IClient {
	return authtokenrequest.NewClient(sdk)
}

func (r *rsaClientDeps) NewCredentials(provider credentials.StaticCredentialsProvider) aws.Credentials {
	value, _ := provider.Retrieve(context.TODO())
	return value

}
