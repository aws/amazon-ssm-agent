/*
 * Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"). You may not
 * use this file except in compliance with the License. A copy of the
 * License is located at
 *
 * http://aws.amazon.com/apache2.0/
 *
 * or in the "license" file accompanying this file. This file is distributed
 * on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

package digest

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func getTestDigestAuthzObj(algorithm string) *Authorization {
	return &Authorization{
		algorithm: algorithm,
		realm:     "realm",
		nonce:     "nonce",
		cnonce:    "cnonce",
		nc:        1,
	}
}

func getTestWwwAuthProps(userhash bool) *WwwAuthenticateProps {
	return &WwwAuthenticateProps{
		realm:     "realm",
		nonce:     "nonce",
		opaque:    "opaque",
		algorithm: MD5,
		qop:       []string{"auth"},
		userhash:  userhash,
	}
}

func TestComputeHash(t *testing.T) {
	tests := []struct {
		algorithm string
		hash      string
		err       error
	}{
		{
			"MD5",
			"661f8009fa8e56a9d0e94a0a644397d7",
			nil,
		},
		{
			"MD5-SESS",
			"661f8009fa8e56a9d0e94a0a644397d7",
			nil,
		},
		{
			"SHA-256",
			"ffe65f1d98fafedea3514adc956c8ada5980c6c5d2552fd61f48401aefd5c00e",
			nil,
		},
		{
			"SHA-256-sess",
			"ffe65f1d98fafedea3514adc956c8ada5980c6c5d2552fd61f48401aefd5c00e",
			nil,
		},
		{
			"SHA512",
			"",
			errors.New("Algorithm SHA512 not supported"),
		},
	}

	for _, test := range tests {
		hash, err := computeHash("test-string", test.algorithm)

		if test.err != nil && assert.Error(t, test.err, "An error was expected") {
			assert.Equal(t, test.err, err)
		} else {
			assert.Equal(t, test.hash, hash)
		}
	}
}

func TestComputeUserhash(t *testing.T) {
	username := "username"
	digestAuthz := getTestDigestAuthzObj(MD5)

	expectedHash, _ := computeHash(fmt.Sprintf("%s:%s", username, digestAuthz.realm), digestAuthz.algorithm)
	actualHash, _ := digestAuthz.computeUserhash(username)
	assert.Equal(t, expectedHash, actualHash)
}

func TestGenerateCNonce(t *testing.T) {
	digestAuthz := getTestDigestAuthzObj(MD5)
	cnonce, _ := digestAuthz.generateCNonce()

	cnonceRegex, _ := regexp.Compile("^[0-9A-Fa-f]{16}$")
	assert.Regexp(t, cnonceRegex, cnonce)
}

func TestComputeHA1(t *testing.T) {
	username := "username"
	password := "password"
	digestAuthz := getTestDigestAuthzObj(MD5)

	md5Hash, _ := computeHash(fmt.Sprintf("%s:%s:%s", username, digestAuthz.realm, password), MD5)
	md5SessHash, _ := computeHash(fmt.Sprintf("%s:%s:%s", md5Hash, digestAuthz.nonce, digestAuthz.cnonce), MD5_SESS)

	tests := []struct {
		algorithm string
		ha1       string
	}{
		{
			MD5,
			md5Hash,
		},
		{
			MD5_SESS,
			md5SessHash,
		},
	}

	for _, test := range tests {
		digestAuthz.algorithm = test.algorithm
		ha1, _ := digestAuthz.computeHA1(username, password)

		assert.Equal(t, test.ha1, ha1)
	}
}

func TestComputeHA2AuthInt(t *testing.T) {
	body := "request-body"
	method := "GET"
	uri := "/index.html"

	digestAuthz := getTestDigestAuthzObj(MD5)
	digestAuthz.qop = "auth-int"

	bodyHash, _ := computeHash(body, digestAuthz.algorithm)
	expectedHA2, _ := computeHash(fmt.Sprintf("%s:%s:%s", method, uri, bodyHash), digestAuthz.algorithm)

	actualHA2, _ := digestAuthz.computeHA2(method, uri, body)
	assert.Equal(t, expectedHA2, actualHA2)
}

func TestComputeHA2Auth(t *testing.T) {
	method := "GET"
	uri := "/index.html"

	digestAuthz := getTestDigestAuthzObj(MD5)
	digestAuthz.qop = "auth"

	expectedHA2, _ := computeHash(fmt.Sprintf("%s:%s", method, uri), digestAuthz.algorithm)

	actualHA2, _ := digestAuthz.computeHA2(method, uri, "")
	assert.Equal(t, expectedHA2, actualHA2)
}

func TestComputeResponse(t *testing.T) {
	username := "username"
	password := "password"
	request := httptest.NewRequest(http.MethodGet, "https://example.com/index.html", strings.NewReader("body"))

	digestAuthz := getTestDigestAuthzObj(MD5)
	digestAuthz.qop = "auth-int"

	ha1, _ := computeHash(
		fmt.Sprintf("%s:%s:%s", username, digestAuthz.realm, password), digestAuthz.algorithm,
	)

	bodyHash, _ := computeHash("body", digestAuthz.algorithm)
	ha2, _ := computeHash(
		fmt.Sprintf("%s:%s:%s", request.Method, request.URL.Path, bodyHash), digestAuthz.algorithm,
	)

	expectedResponse, _ := computeHash(fmt.Sprintf(
		"%s:%s:%08x:%s:%s:%s",
		ha1,
		digestAuthz.nonce,
		digestAuthz.nc,
		digestAuthz.cnonce,
		digestAuthz.qop,
		ha2,
	), digestAuthz.algorithm)

	actualResponse, _ := digestAuthz.computeResponse(username, password, request)
	assert.Equal(t, expectedResponse, actualResponse)
}

func TestString(t *testing.T) {
	digestAuthz := &Authorization{
		username:  "username",
		realm:     "realm",
		nonce:     "nonce",
		uri:       "uri",
		response:  "response",
		algorithm: "algorithm",
		cnonce:    "cnonce",
		opaque:    "opaque",
		qop:       "qop",
		nc:        1,
		userhash:  false,
	}

	expectedString := `Digest username="username", realm="realm", nonce="nonce", uri="uri", response=response, ` +
		`algorithm=algorithm, cnonce="cnonce", opaque="opaque", qop=qop, nc=00000001, userhash=false`

	assert.Equal(t, expectedString, digestAuthz.String())
}

func TestUserhashNewDigestAuthorization(t *testing.T) {
	authProps := getTestWwwAuthProps(true)

	username := "username"
	userhash, _ := computeHash(fmt.Sprintf("%s:%s", username, authProps.realm), MD5)

	tests := []struct {
		withUserhash  bool
		usernameField string
	}{
		{
			withUserhash:  false,
			usernameField: username,
		},
		{
			withUserhash:  true,
			usernameField: userhash,
		},
	}

	for _, test := range tests {
		authProps.userhash = test.withUserhash
		authorization, _ := newDigestAuthorization(
			username,
			"",
			httptest.NewRequest(http.MethodGet, "https://example.com", nil),
			authProps,
		)

		assert.Equal(t, test.withUserhash, authorization.userhash)
		assert.Equal(t, test.usernameField, authorization.username)
	}

}

func TestNewDigestAuthorization(t *testing.T) {
	authProps := getTestWwwAuthProps(true)

	username := "username"
	password := "password"
	request := httptest.NewRequest(http.MethodGet, "https://example.com/index.html", strings.NewReader("body"))
	digestAuthz, _ := newDigestAuthorization(username, password, request, authProps)

	ha1, _ := computeHash(
		fmt.Sprintf("%s:%s:%s", username, authProps.realm, password), authProps.algorithm,
	)

	ha2, _ := computeHash(
		fmt.Sprintf("%s:%s", request.Method, request.URL.Path), authProps.algorithm,
	)
	expectedResponse, _ := computeHash(fmt.Sprintf(
		"%s:%s:%08x:%s:%s:%s",
		ha1,
		authProps.nonce,
		1,
		digestAuthz.cnonce,
		digestAuthz.qop,
		ha2,
	), digestAuthz.algorithm)

	assert.Equal(t, expectedResponse, digestAuthz.response)
	assert.Equal(t, authProps.userhash, digestAuthz.userhash)
	assert.Equal(t, authProps.realm, digestAuthz.realm)
	assert.Equal(t, authProps.algorithm, digestAuthz.algorithm)
	assert.Equal(t, authProps.qop[0], digestAuthz.qop)
	assert.Equal(t, authProps.nonce, digestAuthz.nonce)
	assert.Equal(t, authProps.opaque, digestAuthz.opaque)
	assert.Equal(t, request.URL.Path, digestAuthz.uri)
	assert.Equal(t, 1, digestAuthz.nc)
	assert.NotEmpty(t, digestAuthz.cnonce)
}

func TestAuthorizeAuthzNotRequired(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	req := httptest.NewRequest(http.MethodGet, testServer.URL, nil)
	req.RequestURI = ""

	authorize, err := Authorize("", "", req, &http.Client{})
	assert.Empty(t, authorize)
	assert.EqualError(t, err, "Unexpected HTTP response code received 200 instead of 401 Unauthorized. The requested resources might not require authorization")
}

func TestAuthorizeAuthz(t *testing.T) {
	wwwAuthenticateStr := `Digest realm="realm", nonce="nonce", qop="auth, auth-int", ` +
		`opaque="opaque", algorithm=MD5, userhash=false`
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Www-Authenticate", wwwAuthenticateStr)
		res.WriteHeader(http.StatusUnauthorized)
	}))
	defer testServer.Close()

	req := httptest.NewRequest(http.MethodGet, testServer.URL, nil)
	req.RequestURI = ""

	testDigestAuthz := &Authorization{}

	var actualUsername string
	var actualPassword string
	var actualReq *http.Request
	var actualAuthProps *WwwAuthenticateProps

	createDigestAuthorization = func(
		username string,
		password string,
		req *http.Request,
		authProps *WwwAuthenticateProps,
	) (*Authorization, error) {
		actualUsername = username
		actualPassword = password
		actualReq = req
		actualAuthProps = authProps

		return &Authorization{}, nil
	}

	authorize, err := Authorize("username", "password", req, &http.Client{})
	assert.NoError(t, err)

	assert.Equal(t, "username", actualUsername)
	assert.Equal(t, "password", actualPassword)
	assert.Equal(t, req, actualReq)
	assert.Equal(t, newWwwAuthenticateProps(wwwAuthenticateStr), actualAuthProps)
	assert.Equal(t, testDigestAuthz.String(), authorize)
}
