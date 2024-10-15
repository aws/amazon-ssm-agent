package envdetect

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect"
	"github.com/stretchr/testify/mock"
)

type CollectorMock struct {
	mock.Mock
}

func (cd *CollectorMock) CollectData(context context.T) (*envdetect.Environment, error) {
	args := cd.Called(context)
	return args.Get(0).(*envdetect.Environment), args.Error(1)
}
