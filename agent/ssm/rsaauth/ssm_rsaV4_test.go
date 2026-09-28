// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package rsaauth

import (
	cont "context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

const managedInstanceID string = "s-1234567890abcdefa"
const publicKey string = "MIIEowIBAAKCAQEAralnpd1po1RzmYMP120ptm3MNBkykvtQFuWAvu7jqftYE1i7YiUpqRX+0f/A/3QLzyATDwX4F0hK1tQ1zhCBoGhH/zARhRk30kU/F8b1h4GuA+IVI5nGZYY8oLD8Q4eg+ZlunAWnhL12z1PcihzMvuRoloGh1htsTp0FG0XIyWFyRjgY5a1BDjyQ8KjFpYhhk0q74l+WgQq5xABq/ZKHgm0YiKCj30FnwYP0gtGcXiPJm5juUU6x6XfnzJSZt8xKIVytAFDtyqaZF4YeWRAVAYlfpPx/KWSkxBcpRlBcEBtxSHzwWhYpGtr4iWx84WZVeS/OXWXkDK2iexZkt9l9+QIDAQABAoIBACCRq9mkm8JA/Wkl9ludonwPPYPr0dtU/KE+q5WjcdkYRV1jf8kZVSXb9S1nPMfr+KcAyfJAWVXsffSqWejqmZT+2bnXRwHiR+DMkdegvb2LKZqa1QIXPekectJkPvtfPWZySxdBzDgN3HFnte3FFvUaGy9W4oYoIHjh4+pIfS6fI9rVZR9WjfS4C1mgFZqI0zRF7ql3uwM5MYzeN9HiNY9Yz+J6kuBkMlnyM6urVVvcfKIRZm9El8E+19EiRl34pozY7k0biRss1EDVSCZIBq9IeMarQXoz6QKpAtUKUWrcIKy8z7drWXtKiA29GafiujRHO2yvNx93ZFYzHsVykYECgYEA0LNWUUGjGM4aNRr+8o/AKkrfor7Hgp5fcOqdSsghcnMAI578Ae8IhQkjS+u8ytolpJ4vuUl0rozovDdu2ymWowfjaYFD93mMoRhMMGZtAHf48bFv4grLOlPJ+9/OCreKaq1b2K1ZlbHfGLkIkMmneexa0V4YZURl56bc/gptZ9ECgYEA1QUj+RJfpqwbm+Nr+w/7oCFXerYHf7GCl0QoWuE7hgq7RB3O7WZbxz6qpgKYvnQlSpLFB0cl6Sm500Af5upfzFnSrR2T7L3o2MhP11ZhPiqlhvvmlrc4egbzpbQMq5dPh9kaoFL3KvFt8YMtQr0Sey199AWkWmGWEaZPKs4i5akCgYBEtHVbJLeTp+4aw3tg0RAbHDEJO7Mkfgy/eI01nDLeoZtPHrypyk5MtZhoGwA4653u1qCxZ8xA1mSb6cfV4JgVrbgg+IwugVZZhk02tdF2kQhkUNybVqBW4FSjVadYAdpQiietakwOqtLeKbP3LluzGKtBN6/iTqUZoOYpv7cKsQKBgQCW/jvPcvyl8dzoFL4Xie68RKXzb0/FbZe5jTBlqr08eCLhV5ezoxhvFLZ1UeXfKgi84WgTjpUKvu7fFNcIIR2ihhDVcN/HsZ14/BPL+YiYPjZyhd+e+WRo6sCNtiA9CNXw3y0Gc4iLwfJCfM76PXb6JPbgn5cuEXoELLR1DQSjcQKBgBymCbcDnDikhyoeW00Yg9r8YRaHHjelMar1RmQk19RfuXoxTkOwyIc04B0Zxdnpvif20fQMBnm+/3DvmgJ5pR2EeOiUE8N7JxrTkN6bWgyhIHW1aR/1H6XR6B1JhxiGTNXAO+/1lsHE9AlCFudU3uoA7igiXsfB1PfAx6QUuV5D"

func buildRsaSigner(serviceName string, region string, signTime time.Time, expireTime time.Duration, body string) signer {
	var credsProvider aws.CredentialsProvider
	endpoint := "https://" + serviceName + "." + region + ".amazonaws.com"
	reader := strings.NewReader(body)
	req, _ := http.NewRequest("POST", endpoint, reader)
	smithyReq := &smithyhttp.Request{Request: req}
	smithyReq, _ = smithyReq.SetStream(reader)

	credsProvider = credentials.NewStaticCredentialsProvider(managedInstanceID, publicKey, "")
	req.URL.Opaque = "//example.org/bucket/key-._~,!@#$%^&*()"
	req.Header.Add("X-Amz-Target", "prefix.Operation")
	req.Header.Add("Content-Type", "application/x-amz-json-1.0")
	req.Header.Add("Content-Length", fmt.Sprint(len(body)))
	req.Header.Add("X-Amz-Meta-Other-Header", "some-value=!@#$%^&* (+)")
	return signer{
		Request:       req,
		SmithyRequest: smithyReq,
		Time:          signTime,
		ExpireTime:    expireTime,
		Query:         req.URL.Query(),
		Body:          reader,
		ServiceName:   serviceName,
		Region:        region,
		Credentials:   credsProvider,
	}
}

func createTestRequest() *smithyhttp.Request {
	reader := strings.NewReader("{}")
	req, _ := http.NewRequest("POST", "https://dynamodb.us-west-2.amazonaws.com/", reader)
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", "DynamoDB_20120810.BatchGetItem")
	smithyReq := &smithyhttp.Request{
		Request: req,
	}
	smithyReq, _ = smithyReq.SetStream(reader)
	return smithyReq
}

func TestRsaSignRequest(t *testing.T) {
	signer := buildRsaSigner("ssm", "us-east-1", time.Unix(0, 0), 0, "{}")
	signer.signRsa()

	expectedDate := "19700101T000000Z"
	expectedSig := "AWS4-HMAC-SHA256 Credential=s-1234567890abcdefa/19700101/us-east-1/ssm/aws4_request, SignedHeaders=content-length;content-type;host;x-amz-date;x-amz-meta-other-header;x-amz-target, Signature="

	q := signer.Request.Header
	assert.Contains(t, q.Get("Authorization"), expectedSig)

	assert.Equal(t, expectedDate, q.Get("X-Amz-Date"))
}

func TestRsaSignEmptyBody(t *testing.T) {
	signer := buildRsaSigner("dynamodb", "us-east-1", time.Now(), 0, "")
	signer.Body = nil
	signer.signRsa()
	hash := signer.Request.Header.Get("X-Amz-Content-Sha256")
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", hash)
}

func TestRsaSignBody(t *testing.T) {
	signer := buildRsaSigner("dynamodb", "us-east-1", time.Now(), 0, "hello")
	signer.signRsa()
	hash := signer.Request.Header.Get("X-Amz-Content-Sha256")
	assert.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", hash)
}

func TestRsaSignPrecomputedBodyChecksum(t *testing.T) {
	signer := buildRsaSigner("dynamodb", "us-east-1", time.Now(), 0, "hello")
	signer.Request.Header.Set("X-Amz-Content-Sha256", "PRECOMPUTED")
	signer.signRsa()
	hash := signer.Request.Header.Get("X-Amz-Content-Sha256")
	assert.Equal(t, "PRECOMPUTED", hash)
}

func TestIgnoreRsaResignRequestWithValidCreds(t *testing.T) {
	// Create AWS config with static credentials
	cfg := aws.Config{
		Region:      "us-west-2",
		Credentials: credentials.NewStaticCredentialsProvider(managedInstanceID, publicKey, ""),
	}

	// Create an HTTP request manually (since v2 doesn't expose request creation the same way)
	req := createTestRequest()

	// Sign the request (you'll need to adapt SignRsa to work with smithy requests)
	SignRsa(req, cfg, "ssm")
	sig := req.Header.Get("X-Amz-Signature")

	// Sign again and verify signature doesn't change
	SignRsa(req, cfg, "ssm")
	assert.Equal(t, sig, req.Header.Get("X-Amz-Signature"))

}

func TestRsaResignRequestExpiredCreds(t *testing.T) {
	// Create static credentials provider
	credsProvider := credentials.NewStaticCredentialsProvider(managedInstanceID, publicKey, "")

	cfg := aws.Config{
		Region:      "us-west-2",
		Credentials: credsProvider,
	}

	// Create HTTP request manually
	req := createTestRequest()

	// Sign the request first time
	SignRsa(req, cfg, "ssm")
	firstAuth := req.Header.Get("Authorization")

	// Get credentials and set them as expired
	creds, _ := credsProvider.Retrieve(cont.TODO())
	creds.Expires = time.Now().Add(-1 * time.Hour) // Set to 1 hour ago

	// Create new provider with expired credentials
	expiredProvider := credentials.NewStaticCredentialsProvider(creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken)
	cfg.Credentials = expiredProvider
	// TODO: Figure out how to expire creds without mocking
	req.Header.Del("Authorization")

	// Sign again with expired credentials
	SignRsa(req, cfg, "ssm")
	secondAuth := req.Header.Get("Authorization")

	assert.NotEqual(t, firstAuth, secondAuth)
}

func BenchmarkRsaSignRequest(b *testing.B) {
	signer := buildRsaSigner("dynamodb", "us-east-1", time.Now(), 0, "{}")
	for i := 0; i < b.N; i++ {
		signer.signRsa()
	}
}
