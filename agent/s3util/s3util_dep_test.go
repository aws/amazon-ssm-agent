package s3util

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
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
	getHeadBucketTransportDelegate = func(log.T, appconfig.SsmagentConfig) http.RoundTripper {
		return trans
	}

	httpProvider := HttpProviderImpl{
		logger: log.NewMockLog(),
	}
	actual, err := httpProvider.Head(bucketUrl)
	assert.Nil(t, err)
	assert.Equal(t, "eu-west-1", actual.Header.Get(bucketRegionHeader))
}

func TestHttpProviderImpl_Head_NoRetryOnCertValidationFailure(t *testing.T) {

	bucketUrl := "https://bucket.with.dots.s3.us-east-1.amazonaws.com"
	errMsg := "x509: certificate is valid for s3.amazonaws.com, *.s3.amazonaws.com, " +
		"*.s3.dualstack.us-east-1.amazonaws.com, s3.dualstack.us-east-1.amazonaws.com, " +
		"*.s3.us-east-1.amazonaws.com, s3.us-east-1.amazonaws.com, *.s3-control.us-east-1.amazonaws.com, " +
		"s3-control.us-east-1.amazonaws.com, *.s3-control.dualstack.us-east-1.amazonaws.com, " +
		"s3-control.dualstack.us-east-1.amazonaws.com, *.s3-accesspoint.us-east-1.amazonaws.com, " +
		"*.s3-accesspoint.dualstack.us-east-1.amazonaws.com, *.s3.us-east-1.vpce.amazonaws.com, " +
		"not bucket.with.dots.s3.us-east-1.amazonaws.com"

	trans := newMockTransport()
	trans.AddTransportError(bucketUrl, fmt.Errorf(errMsg))
	getHeadBucketTransportDelegate = func(log.T, appconfig.SsmagentConfig) http.RoundTripper {
		return trans
	}

	httpProvider := HttpProviderImpl{
		logger: log.NewMockLog(),
	}
	actual, err := httpProvider.Head(bucketUrl)
	assert.Nil(t, actual)
	assert.Contains(t, err.Error(), errMsg)
	assert.Equal(t, 1, len(trans.requestURLsReceived))
}
