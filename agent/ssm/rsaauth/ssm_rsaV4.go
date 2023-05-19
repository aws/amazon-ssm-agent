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
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/managedInstances/auth"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
)

const (
	// SsmAuthHeader is the header that holds private key signature for iir-rsa signed requests
	SsmAuthHeader = "SSM-AsymmetricKeyAuthorization"
)

// Sign requests with Beagle RSA using signature version 4.
//
// Will sign the requests with the service config's Credentials object
// The credentials.AccessKeyID is the server id
// The credentials.SecretAccessKey is the 64bit encoded private rsa key

func SignRsa(req *request.Request) {
	// If the request does not need to be signed ignore the signing of the
	// request if the AnonymousCredentials object is used.
	if req.Config.Credentials == credentials.AnonymousCredentials {
		return
	}

	region := req.ClientInfo.SigningRegion
	if region == "" {
		region = aws.StringValue(req.Config.Region)
	}

	name := req.ClientInfo.SigningName
	if name == "" {
		name = req.ClientInfo.ServiceName
	}

	s := signer{
		Request:     req.HTTPRequest,
		Time:        req.Time,
		ExpireTime:  req.ExpireTime,
		Query:       req.HTTPRequest.URL.Query(),
		Body:        req.Body,
		ServiceName: name,
		Region:      region,
		Credentials: req.Config.Credentials,
		Debug:       req.Config.LogLevel.Value(),
		Logger:      req.Config.Logger,
		notHoist:    req.NotHoist,
	}

	req.Error = s.signRsa()
	req.SignedHeaderVals = s.signedHeaderVals
}

func (v4 *signer) signRsa() error {
	if v4.ExpireTime != 0 {
		v4.isPresign = true
	}

	if v4.isRequestSigned() {
		if !v4.Credentials.IsExpired() {
			// If the request is already signed, and the credentials have not
			// expired yet ignore the signing request.
			return nil
		}

		// The credentials have expired for this request. The current signing
		// is invalid, and needs to be request because the request will fail.
		if v4.isPresign {
			v4.removePresign()
			// Update the request's query string to ensure the values stays in
			// sync in the case retrieving the new credentials fails.
			v4.Request.URL.RawQuery = v4.Query.Encode()
		}
	}

	var err error
	v4.CredValues, err = v4.Credentials.Get()
	if err != nil {
		return err
	}

	if v4.isPresign {
		v4.Query.Set("X-Amz-Algorithm", authHeaderPrefix)
		if v4.CredValues.SessionToken != "" {
			v4.Query.Set("X-Amz-Security-Token", v4.CredValues.SessionToken)
		} else {
			v4.Query.Del("X-Amz-Security-Token")
		}
	} else if v4.CredValues.SessionToken != "" {
		v4.Request.Header.Set("X-Amz-Security-Token", v4.CredValues.SessionToken)
	}

	v4.buildRsa()

	if v4.Debug.Matches(aws.LogDebugWithSigning) {
		v4.logSigningInfo()
	}

	return nil
}

func (v4 *signer) buildRsa() {

	v4.buildTime()             // no depends
	v4.buildCredentialString() // no depends

	unsignedHeaders := v4.Request.Header
	if v4.isPresign {
		if !v4.notHoist {
			urlValues := url.Values{}
			urlValues, unsignedHeaders = buildQuery(allowedQueryHoisting, unsignedHeaders) // no depends
			for k := range urlValues {
				v4.Query[k] = urlValues[k]
			}
		}
	}

	v4.buildCanonicalHeaders(ignoredHeaders, unsignedHeaders)
	v4.buildCanonicalString() // depends on canon headers / signed headers
	v4.buildStringToSign()    // depends on canon string
	v4.buildRsaSignature()    // depends on string to sign

	if v4.isPresign {
		v4.Request.URL.RawQuery += "&X-Amz-Signature=" + v4.signature
	} else {
		parts := []string{
			authHeaderPrefix + " Credential=" + v4.CredValues.AccessKeyID + "/" + v4.credentialString,
			"SignedHeaders=" + v4.signedHeaders,
			"Signature=" + v4.signature,
		}
		v4.Request.Header.Set("Authorization", strings.Join(parts, ", "))
	}
}

// Sign the stringToSign using the private key
func (v4 *signer) buildRsaSignature() (err error) {
	v4.signature, err = BuildRSASignature(v4.CredValues.SecretAccessKey, v4.stringToSign)
	return
}

// MakeSignRsaHandler creates an http handler that signs the request using an RSA private key
func MakeSignRsaHandler(encodedPrivateKey string) func(req *request.Request) {
	return func(req *request.Request) {
		authZHeader := req.HTTPRequest.Header.Get("Authorization")
		if len(authZHeader) == 0 {
			req.Error = fmt.Errorf("unable to build RSA signature. No Authorization header in request")
			return
		}
		signature, err := BuildRSASignature(encodedPrivateKey, authZHeader)
		if err != nil {
			req.Error = fmt.Errorf("failed to build RSA signature. Err: %v", err)
			return
		}

		req.HTTPRequest.Header[SsmAuthHeader] = []string{fmt.Sprintf("Signature=%s", signature)}
	}
}

// BuildRSASignature signs a string using a private RSA signing key
func BuildRSASignature(encodedPrivateKey string, stringToSign string) (signature string, err error) {
	var rsaKey auth.RsaKey
	rsaKey, err = auth.DecodePrivateKey(encodedPrivateKey)
	if err != nil {
		return
	}

	signature, err = rsaKey.Sign(stringToSign)
	return
}
