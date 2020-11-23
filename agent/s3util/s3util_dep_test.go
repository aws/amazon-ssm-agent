package s3util

import (
	"net/http"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

func TestHttpProviderImpl_Head_Handles301WithNoLocation(t *testing.T) {
	bucketUrl := "https://bucket-1.s3.us-east-1.amazonaws.com"
	header := http.Header{}
	header.Add(bucketRegionHeader, "eu-west-1")
	resp := &http.Response{
		StatusCode: 301,
		Header:     header,
		Body:       http.NoBody,
	}
	trans := newMockTransport()
	trans.AddResponse(bucketUrl, resp)
	getHeadBucketTransportDelegate = func(log.T) http.RoundTripper {
		return trans
	}

	httpProvider := HttpProviderImpl{
		logger: log.NewMockLog(),
	}
	actual, err := httpProvider.Head(bucketUrl)
	assert.Nil(t, err)
	assert.Equal(t, "eu-west-1", actual.Header.Get(bucketRegionHeader))
}
