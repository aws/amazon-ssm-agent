/* This interface is created manally based on the ssmiface.SSMAPI. In order to keep in one place the only APIs needed for birdwatcher. */
package facade

import (
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// BirdwatcherFacade is the interface type for ssmiface.SSMAPI
type BirdwatcherFacade interface {
	GetManifestRequest(*ssm.GetManifestInput) (*request.Request, *ssm.GetManifestOutput)

	GetManifest(*ssm.GetManifestInput) (*ssm.GetManifestOutput, error)

	PutConfigurePackageResultRequest(*ssm.PutConfigurePackageResultInput) (*request.Request, *ssm.PutConfigurePackageResultOutput)

	PutConfigurePackageResult(*ssm.PutConfigurePackageResultInput) (*ssm.PutConfigurePackageResultOutput, error)
}

var _ BirdwatcherFacade = (*ssm.SSM)(nil)
