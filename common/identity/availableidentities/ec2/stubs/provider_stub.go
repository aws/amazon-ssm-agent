package stubs

import (
	"context"
	"time"

	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ec2roleprovider"
	"github.com/aws/aws-sdk-go-v2/aws"
)

type ProviderStub struct {
	ProviderName  string
	Profile       string
	File          string
	SharesCreds   bool
	InnerProvider ec2roleprovider.IInnerProvider
	Expiry        time.Time
}

func (p *ProviderStub) SetExpiration(expiration time.Time, window time.Duration) {
	return
}

func (p *ProviderStub) Retrieve(ctx context.Context) (aws.Credentials, error) {
	return aws.Credentials{
		Source: p.ProviderName,
	}, nil
}

func (p *ProviderStub) RemoteRetrieve(ctx context.Context) (aws.Credentials, error) {
	return aws.Credentials{
		Source: p.ProviderName,
	}, nil
}

func (p *ProviderStub) IsExpired() bool {
	return false
}

func (p *ProviderStub) ExpiresAt() time.Time {
	return p.Expiry
}

func (p *ProviderStub) ShareProfile() string {
	return p.Profile
}

func (p *ProviderStub) ShareFile() string {
	return p.File
}

func (p *ProviderStub) SharesCredentials() bool {
	return p.SharesCreds
}

func (p *ProviderStub) GetInnerProvider() ec2roleprovider.IInnerProvider {
	return p.InnerProvider
}
