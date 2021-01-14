package envdetect

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/stretchr/testify/mock"
)

type CollectorMock struct {
	mock.Mock
}

func (cd *CollectorMock) CollectData(context context.T) (*Environment, error) {
	args := cd.Called(context)
	return args.Get(0).(*Environment), args.Error(1)
}
