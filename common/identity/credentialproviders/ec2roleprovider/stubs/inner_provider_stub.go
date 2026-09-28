package stubs

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

type InnerProvider struct {
	Credentials  aws.Credentials
	RetrieveErr  error
	ProviderName string
	Expiry       time.Time
}

func (p *InnerProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	if p.RetrieveErr != nil {
		return aws.Credentials{Source: p.ProviderName}, p.RetrieveErr
	}

	creds := aws.Credentials{Source: p.ProviderName, Expires: p.Expiry}

	p.Credentials.Expires = creds.Expires
	p.Credentials.CanExpire = true

	return creds, nil
}

func (p *InnerProvider) IsExpired() bool {
	return p.RetrieveErr != nil
}

func (p *InnerProvider) ExpiresAt() time.Time {
	return p.Expiry
}

func (p *InnerProvider) SetExpiration(expiration time.Time, window time.Duration) {
	p.Credentials.Expires = expiration.Add(-window)
	p.Expiry = p.Credentials.Expires
}
