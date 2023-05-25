package stubs

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
)

type InnerProvider struct {
	RetrieveErr  error
	ProviderName string
	Expiry       time.Time
}

func (p *InnerProvider) RetrieveWithContext(ctx context.Context) (credentials.Value, error) {
	if p.RetrieveErr != nil {
		return credentials.Value{}, p.RetrieveErr
	}

	return credentials.Value{
		ProviderName: p.ProviderName,
	}, nil
}

func (p *InnerProvider) Retrieve() (credentials.Value, error) {
	return p.RetrieveWithContext(context.Background())
}

func (p *InnerProvider) IsExpired() bool {
	return p.RetrieveErr != nil
}

func (p *InnerProvider) ExpiresAt() time.Time {
	return p.Expiry
}

func (p *InnerProvider) SetExpiration(expiration time.Time, window time.Duration) {
	p.Expiry = expiration.Add(-window)
}
