package envdetect

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/mock"
)

type CollectorMock struct {
	mock.Mock
}

func (cd *CollectorMock) CollectData(log log.T) (*Environment, error) {
	args := cd.Called(log)
	return args.Get(0).(*Environment), args.Error(1)
}
