package stubs

import (
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
)

const (
	SharedProviderName    = "SharedProvider"
	NonSharedProviderName = "NonSharedProvider"
)

type InnerProvider struct {
	ProviderName string
	Expiry       time.Time
}

func (p *InnerProvider) Retrieve() (credentials.Value, error) {
	return credentials.Value{
		ProviderName: p.ProviderName,
	}, nil
}

func (p *InnerProvider) IsExpired() bool {
	return false
}

func (p *InnerProvider) ExpiresAt() time.Time {
	return p.Expiry
}
