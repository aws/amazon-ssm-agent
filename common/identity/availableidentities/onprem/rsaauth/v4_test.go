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

// Copied from github.com/aws/aws-sdk-go/private/signer/v4
// to provide common SigV4 dependenies for the RSA signer.

package rsaauth

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/awstesting"
)

func buildSigner(serviceName string, region string, signTime time.Time, expireTime time.Duration, body string) signer {
	endpoint := "https://" + serviceName + "." + region + ".amazonaws.com"
	reader := strings.NewReader(body)
	req, _ := http.NewRequest("POST", endpoint, reader)
	req.URL.Opaque = "//example.org/bucket/key-._~,!@#$%^&*()"
	req.Header.Add("X-Amz-Target", "prefix.Operation")
	req.Header.Add("Content-Type", "application/x-amz-json-1.0")
	req.Header.Add("Content-Length", fmt.Sprint(len(body)))
	req.Header.Add("X-Amz-Meta-Other-Header", "some-value=!@#$%^&* (+)")
	req.Header.Add("X-Amz-Meta-Other-Header_With_Underscore", "some-value=!@#$%^&* (+)")
	req.Header.Add("X-amz-Meta-Other-Header_With_Underscore", "some-value=!@#$%^&* (+)")

	return signer{
		Request:     req,
		Time:        signTime,
		ExpireTime:  expireTime,
		Query:       req.URL.Query(),
		Body:        reader,
		ServiceName: serviceName,
		Region:      region,
		Credentials: credentials.NewStaticCredentials("AKID", "SECRET", "SESSION"),
	}
}

func removeWS(text string) string {
	text = strings.Replace(text, " ", "", -1)
	text = strings.Replace(text, "\n", "", -1)
	text = strings.Replace(text, "\t", "", -1)
	return text
}

func assertEqual(t *testing.T, expected, given string) {
	if removeWS(expected) != removeWS(given) {
		t.Errorf("\nExpected: %s\nGiven:    %s", expected, given)
	}
}

func TestPresignRequest(t *testing.T) {
	signer := buildSigner("dynamodb", "us-east-1", time.Unix(0, 0), 300*time.Second, "{}")
	signer.sign()

	expectedDate := "19700101T000000Z"
	expectedHeaders := "content-length;content-type;host;x-amz-meta-other-header;x-amz-meta-other-header_with_underscore"
	expectedSig := "122f0b9e091e4ba84286097e2b3404a1f1f4c4aad479adda95b7dff0ccbe5581"
	expectedCred := "AKID/19700101/us-east-1/dynamodb/aws4_request"
	expectedTarget := "prefix.Operation"

	q := signer.Request.URL.Query()
	assert.Equal(t, expectedSig, q.Get("X-Amz-Signature"))
	assert.Equal(t, expectedCred, q.Get("X-Amz-Credential"))
	assert.Equal(t, expectedHeaders, q.Get("X-Amz-SignedHeaders"))
	assert.Equal(t, expectedDate, q.Get("X-Amz-Date"))
	assert.Empty(t, q.Get("X-Amz-Meta-Other-Header"))
	assert.Equal(t, expectedTarget, q.Get("X-Amz-Target"))
}

func TestSignRequest(t *testing.T) {
	signer := buildSigner("dynamodb", "us-east-1", time.Unix(0, 0), 0, "{}")
	signer.sign()

	expectedDate := "19700101T000000Z"
	expectedSig := "AWS4-HMAC-SHA256 Credential=AKID/19700101/us-east-1/dynamodb/aws4_request, SignedHeaders=content-length;content-type;host;x-amz-date;x-amz-meta-other-header;x-amz-meta-other-header_with_underscore;x-amz-security-token;x-amz-target, Signature=a518299330494908a70222cec6899f6f32f297f8595f6df1776d998936652ad9"

	q := signer.Request.Header
	assert.Equal(t, expectedSig, q.Get("Authorization"))
	assert.Equal(t, expectedDate, q.Get("X-Amz-Date"))
}

func TestSignEmptyBody(t *testing.T) {
	signer := buildSigner("dynamodb", "us-east-1", time.Now(), 0, "")
	signer.Body = nil
	signer.sign()
	hash := signer.Request.Header.Get("X-Amz-Content-Sha256")
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", hash)
}

func TestSignBody(t *testing.T) {
	signer := buildSigner("dynamodb", "us-east-1", time.Now(), 0, "hello")
	signer.sign()
	hash := signer.Request.Header.Get("X-Amz-Content-Sha256")
	assert.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", hash)
}

func TestSignSeekedBody(t *testing.T) {
	signer := buildSigner("dynamodb", "us-east-1", time.Now(), 0, "   hello")
	signer.Body.Read(make([]byte, 3)) // consume first 3 bytes so body is now "hello"
	signer.sign()
	hash := signer.Request.Header.Get("X-Amz-Content-Sha256")
	assert.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", hash)

	start, _ := signer.Body.Seek(0, 1)
	assert.Equal(t, int64(3), start)
}

func TestPresignEmptyBodyS3(t *testing.T) {
	signer := buildSigner("s3", "us-east-1", time.Now(), 5*time.Minute, "hello")
	signer.sign()
	hash := signer.Request.Header.Get("X-Amz-Content-Sha256")
	assert.Equal(t, "UNSIGNED-PAYLOAD", hash)
}

func TestSignPrecomputedBodyChecksum(t *testing.T) {
	signer := buildSigner("dynamodb", "us-east-1", time.Now(), 0, "hello")
	signer.Request.Header.Set("X-Amz-Content-Sha256", "PRECOMPUTED")
	signer.sign()
	hash := signer.Request.Header.Get("X-Amz-Content-Sha256")
	assert.Equal(t, "PRECOMPUTED", hash)
}

func TestAnonymousCredentials(t *testing.T) {
	svc := awstesting.NewClient(&aws.Config{Credentials: credentials.AnonymousCredentials})
	r := svc.NewRequest(
		&request.Operation{
			Name:       "BatchGetItem",
			HTTPMethod: "POST",
			HTTPPath:   "/",
		},
		nil,
		nil,
	)
	sign(r)

	urlQ := r.HTTPRequest.URL.Query()
	assert.Empty(t, urlQ.Get("X-Amz-Signature"))
	assert.Empty(t, urlQ.Get("X-Amz-Credential"))
	assert.Empty(t, urlQ.Get("X-Amz-SignedHeaders"))
	assert.Empty(t, urlQ.Get("X-Amz-Date"))

	hQ := r.HTTPRequest.Header
	assert.Empty(t, hQ.Get("Authorization"))
	assert.Empty(t, hQ.Get("X-Amz-Date"))
}

func TestIgnoreResignRequestWithValidCreds(t *testing.T) {
	svc := awstesting.NewClient(&aws.Config{
		Credentials: credentials.NewStaticCredentials("AKID", "SECRET", "SESSION"),
		Region:      aws.String("us-west-2"),
	})
	r := svc.NewRequest(
		&request.Operation{
			Name:       "BatchGetItem",
			HTTPMethod: "POST",
			HTTPPath:   "/",
		},
		nil,
		nil,
	)

	sign(r)
	sig := r.HTTPRequest.Header.Get("Authorization")

	sign(r)
	assert.Equal(t, sig, r.HTTPRequest.Header.Get("Authorization"))
}

func TestIgnorePreResignRequestWithValidCreds(t *testing.T) {
	svc := awstesting.NewClient(&aws.Config{
		Credentials: credentials.NewStaticCredentials("AKID", "SECRET", "SESSION"),
		Region:      aws.String("us-west-2"),
	})
	r := svc.NewRequest(
		&request.Operation{
			Name:       "BatchGetItem",
			HTTPMethod: "POST",
			HTTPPath:   "/",
		},
		nil,
		nil,
	)
	r.ExpireTime = time.Minute * 10

	sign(r)
	sig := r.HTTPRequest.Header.Get("X-Amz-Signature")

	sign(r)
	assert.Equal(t, sig, r.HTTPRequest.Header.Get("X-Amz-Signature"))
}

func TestResignRequestExpiredCreds(t *testing.T) {
	creds := credentials.NewStaticCredentials("AKID", "SECRET", "SESSION")
	svc := awstesting.NewClient(&aws.Config{Credentials: creds})
	r := svc.NewRequest(
		&request.Operation{
			Name:       "BatchGetItem",
			HTTPMethod: "POST",
			HTTPPath:   "/",
		},
		nil,
		nil,
	)
	sign(r)
	querySig := r.HTTPRequest.Header.Get("Authorization")
	var origSignedHeaders string
	for _, p := range strings.Split(querySig, ", ") {
		if strings.HasPrefix(p, "SignedHeaders=") {
			origSignedHeaders = p[len("SignedHeaders="):]
			break
		}
	}
	assert.NotEmpty(t, origSignedHeaders)
	assert.NotContains(t, origSignedHeaders, "authorization")

	creds.Expire()

	sign(r)
	updatedQuerySig := r.HTTPRequest.Header.Get("Authorization")
	assert.NotEqual(t, querySig, updatedQuerySig)

	var updatedSignedHeaders string
	for _, p := range strings.Split(updatedQuerySig, ", ") {
		if strings.HasPrefix(p, "SignedHeaders=") {
			updatedSignedHeaders = p[len("SignedHeaders="):]
			break
		}
	}
	assert.NotEmpty(t, updatedSignedHeaders)
	assert.NotContains(t, updatedQuerySig, "authorization")
}

func TestPreResignRequestExpiredCreds(t *testing.T) {
	provider := &credentials.StaticProvider{Value: credentials.Value{
		AccessKeyID:     "AKID",
		SecretAccessKey: "SECRET",
		SessionToken:    "SESSION",
	}}
	creds := credentials.NewCredentials(provider)
	svc := awstesting.NewClient(&aws.Config{Credentials: creds})
	r := svc.NewRequest(
		&request.Operation{
			Name:       "BatchGetItem",
			HTTPMethod: "POST",
			HTTPPath:   "/",
		},
		nil,
		nil,
	)
	r.ExpireTime = time.Minute * 10

	sign(r)
	querySig := r.HTTPRequest.URL.Query().Get("X-Amz-Signature")
	signedHeaders := r.HTTPRequest.URL.Query().Get("X-Amz-SignedHeaders")
	assert.NotEmpty(t, signedHeaders)

	creds.Expire()
	r.Time = time.Now().Add(time.Hour * 48)

	sign(r)
	assert.NotEqual(t, querySig, r.HTTPRequest.URL.Query().Get("X-Amz-Signature"))
	resignedHeaders := r.HTTPRequest.URL.Query().Get("X-Amz-SignedHeaders")
	assert.Equal(t, signedHeaders, resignedHeaders)
	assert.NotContains(t, signedHeaders, "x-amz-signedHeaders")
}

func TestResignRequestExpiredRequest(t *testing.T) {
	creds := credentials.NewStaticCredentials("AKID", "SECRET", "SESSION")
	svc := awstesting.NewClient(&aws.Config{Credentials: creds})
	r := svc.NewRequest(
		&request.Operation{
			Name:       "BatchGetItem",
			HTTPMethod: "POST",
			HTTPPath:   "/",
		},
		nil,
		nil,
	)

	sign(r)
	querySig := r.HTTPRequest.Header.Get("Authorization")

	// Simulate the request occured 15 minutes in the past
	r.Time = r.Time.Add(-15 * time.Minute)

	sign(r)
	assert.NotEqual(t, querySig, r.HTTPRequest.Header.Get("Authorization"))
}

func TestStripExcessHeaders(t *testing.T) {
	vals := []string{
		"123",
		"1 2 3",
		"  1 2 3",
		"1  2 3",
		"1  23",
		"1  2  3",
		"1  2  ",
		" 1  2  ",
	}

	expected := []string{
		"123",
		"1 2 3",
		"1 2 3",
		"1 2 3",
		"1 23",
		"1 2 3",
		"1 2",
		"1 2",
	}

	newVals := stripExcessSpaces(vals)
	for i := 0; i < len(newVals); i++ {
		assert.Equal(t, newVals[i], expected[i])
	}
}

func BenchmarkPresignRequest(b *testing.B) {
	signer := buildSigner("dynamodb", "us-east-1", time.Now(), 300*time.Second, "{}")
	for i := 0; i < b.N; i++ {
		signer.sign()
	}
}

func BenchmarkSignRequest(b *testing.B) {
	signer := buildSigner("dynamodb", "us-east-1", time.Now(), 0, "{}")
	for i := 0; i < b.N; i++ {
		signer.sign()
	}
}
