package stubs

import (
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
)

type InnerProvider struct {
	RetrieveErr  error
	ProviderName string
	Expiry       time.Time
}

func (p *InnerProvider) Retrieve() (credentials.Value, error) {
	if p.RetrieveErr != nil {
		return credentials.Value{}, p.RetrieveErr
	}

	return credentials.Value{
		ProviderName: p.ProviderName,
	}, nil
}

func (p *InnerProvider) IsExpired() bool {
	if p.RetrieveErr != nil {
		return true
	}

	return false
}

func (p *InnerProvider) ExpiresAt() time.Time {
	return p.Expiry
}
