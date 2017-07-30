package ec2infradetect

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/mock"
)

type platfromProviderMock struct {
	mock.Mock
}

func (ds *platfromProviderMock) InstanceID(log log.T) (string, error) {
	args := ds.Called(log)
	return args.String(0), args.Error(1)
}

func (ds *platfromProviderMock) InstanceType(log log.T) (string, error) {
	args := ds.Called(log)
	return args.String(0), args.Error(1)
}

func (ds *platfromProviderMock) AvailabilityZone(log log.T) (string, error) {
	args := ds.Called(log)
	return args.String(0), args.Error(1)
}

func (ds *platfromProviderMock) Region(log log.T) (string, error) {
	args := ds.Called(log)
	return args.String(0), args.Error(1)
}
