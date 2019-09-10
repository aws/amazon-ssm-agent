package s3util

import (
	"net/http"
	"testing"

	"errors"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	FakeS3Endpoint       = "s3.amazonaws.com/ssmagent/test"
	expectedRegionHeader = "X-Amz-Bucket-Region"
)

func TestGetRegionFromS3URLWithExponentialBackoff(t *testing.T) {
	expectedResp := http.Response{
		StatusCode: 200,
		Header:     http.Header{expectedRegionHeader: []string{"us-east-1"}},
	}

	mockHttpProvider := MockedHttpProvider{}
	mockHttpProvider.On("Head", FakeS3Endpoint).Return(&expectedResp, nil).Once()

	region, err := getRegionFromS3URLWithExponentialBackoff(FakeS3Endpoint, &mockHttpProvider)

	assert.Equal(t, "us-east-1", region)
	assert.Nil(t, err)
	mockHttpProvider.AssertExpectations(t)
}

func TestGetRegionFromS3URLWithExponentialBackoff_HeadReturnsErrors(t *testing.T) {
	expectedResp := http.Response{
		StatusCode: 404,
		Header:     http.Header{},
	}

	mockHttpProvider := MockedHttpProvider{}
	mockHttpProvider.On("Head", FakeS3Endpoint).Return(&expectedResp, errors.New("Expected error occurred"))

	region, err := getRegionFromS3URLWithExponentialBackoff(FakeS3Endpoint, &mockHttpProvider)

	assert.Empty(t, region)
	assert.NotNil(t, err)
	mockHttpProvider.AssertExpectations(t)
}

func TestGetRegionFromS3URLWithExponentialBackoff_HeadReturnsErrorTwice(t *testing.T) {
	expectedResp1 := http.Response{
		StatusCode: 404,
		Header:     http.Header{},
	}
	expectedResp2 := http.Response{
		StatusCode: 200,
		Header:     http.Header{expectedRegionHeader: []string{"us-east-1"}},
	}

	mockHttpProvider := MockedHttpProvider{}
	mockHttpProvider.On("Head", FakeS3Endpoint).Return(&expectedResp1, errors.New("Expected error occurred")).Twice()
	mockHttpProvider.On("Head", FakeS3Endpoint).Return(&expectedResp2, nil).Once()

	region, err := getRegionFromS3URLWithExponentialBackoff(FakeS3Endpoint, &mockHttpProvider)

	assert.Equal(t, "us-east-1", region)
	assert.Nil(t, err)
	mockHttpProvider.AssertExpectations(t)
}

func TestGetRegionFromS3URLWithExponentialBackoff_HeadReturns503Twice(t *testing.T) {
	expectedResp1 := http.Response{
		StatusCode: 503,
		Header:     http.Header{},
	}
	expectedResp2 := http.Response{
		StatusCode: 200,
		Header:     http.Header{expectedRegionHeader: []string{"us-east-1"}},
	}

	mockHttpProvider := MockedHttpProvider{}
	mockHttpProvider.On("Head", FakeS3Endpoint).Return(&expectedResp1, nil).Twice()
	mockHttpProvider.On("Head", FakeS3Endpoint).Return(&expectedResp2, nil).Once()

	region, err := getRegionFromS3URLWithExponentialBackoff(FakeS3Endpoint, &mockHttpProvider)

	assert.Equal(t, "us-east-1", region)
	assert.Nil(t, err)
	mockHttpProvider.AssertExpectations(t)
}

func TestGetRegionFromS3URLWithExponentialBackoff_HeadReturns503s(t *testing.T) {
	expectedResp := http.Response{
		StatusCode: 503,
		Header:     http.Header{},
	}

	mockHttpProvider := MockedHttpProvider{}
	mockHttpProvider.On("Head", FakeS3Endpoint).Return(&expectedResp, nil)

	region, err := getRegionFromS3URLWithExponentialBackoff(FakeS3Endpoint, &mockHttpProvider)

	assert.Empty(t, region)
	assert.NotNil(t, err)
	mockHttpProvider.AssertExpectations(t)
}

func TestGetRegionFromS3URLSucceed(t *testing.T) {
	expectedResp := http.Response{
		StatusCode: 200,
		Header:     http.Header{expectedRegionHeader: []string{"us-west-1"}},
	}

	log := log.NewMockLog()
	mockHttpProvider := MockedHttpProvider{}
	mockHttpProvider.On("Head", FakeS3Endpoint).Return(&expectedResp, nil).Once()

	region, err := getRegionFromS3URL(log, FakeS3Endpoint, &mockHttpProvider)

	assert.Equal(t, "us-west-1", region)
	assert.Nil(t, err)
	mockHttpProvider.AssertExpectations(t)
}

func TestGetRegionFromS3URLFailedReturn404Once(t *testing.T) {
	log := log.NewMockLog()
	expectedResp1 := http.Response{
		StatusCode: 404,
		Header:     http.Header{},
	}
	expectedResp2 := http.Response{
		StatusCode: 200,
		Header:     http.Header{expectedRegionHeader: []string{"us-west-2"}},
	}

	mockHttpProvider := MockedHttpProvider{}
	mockHttpProvider.On("Head", FakeS3Endpoint).Return(&expectedResp1, nil).Once()
	mockHttpProvider.On("Head", FakeS3Endpoint).Return(&expectedResp2, nil).Once()

	region, err := getRegionFromS3URL(log, FakeS3Endpoint, &mockHttpProvider)
	assert.Empty(t, region)
	assert.NotNil(t, err)

	region, err = getRegionFromS3URL(log, FakeS3Endpoint, &mockHttpProvider)
	assert.Equal(t, "us-west-2", region)
	assert.Nil(t, err)
	mockHttpProvider.AssertExpectations(t)
}

func TestGetRegionFromS3URLFailedReturn503Once(t *testing.T) {
	log := log.NewMockLog()
	expectedResp1 := http.Response{
		StatusCode: 503,
		Header:     http.Header{},
	}

	mockHttpProvider := MockedHttpProvider{}
	mockHttpProvider.On("Head", FakeS3Endpoint).Return(&expectedResp1, nil).Once()

	region, err := getRegionFromS3URL(log, FakeS3Endpoint, &mockHttpProvider)
	assert.Empty(t, region)
	assert.NotNil(t, err)
	mockHttpProvider.AssertExpectations(t)
}

type MockedHttpProvider struct {
	mock.Mock
}

func (m *MockedHttpProvider) Head(url string) (*http.Response, error) {
	args := m.Called(url)
	return args.Get(0).(*http.Response), args.Error(1)
}
