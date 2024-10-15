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

// Package digest defines functionality required to support digest authorization
package digest

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

var (
	md5Writer    = md5.New()
	sha256Writer = sha256.New()
)

// Possible values of the qop parameter
const (
	QOP_AUTH     = "auth"
	QOP_AUTH_INT = "auth-int"
)

// Supported encryption algorithms
const (
	MD5         = "MD5"
	MD5_SESS    = "MD5-SESS"
	SHA256      = "SHA-256"
	SHA256_SESS = "SHA-256-SESS"
)

var createDigestAuthorization = newDigestAuthorization

// Authorization defines all required attributes for the "Authorization" request header when performing digest authz
// https://tools.ietf.org/html/rfc7616#section-3.4
type Authorization struct {
	username  string `quoted:"true"`
	realm     string `quoted:"true"`
	nonce     string `quoted:"true"`
	uri       string `quoted:"true"`
	response  string `quoted:"false"`
	algorithm string `quoted:"false"`
	cnonce    string `quoted:"true"`
	opaque    string `quoted:"true"`
	qop       string `quoted:"false"`
	nc        int    `quoted:"false"`
	userhash  bool   `quoted:"false"`
}

// newDigestAuthorization creates a new Authorization instance and computes all required fields
func newDigestAuthorization(
	username string,
	password string,
	req *http.Request,
	authProps *WwwAuthenticateProps,
) (*Authorization, error) {
	authz := &Authorization{
		realm:     authProps.realm,
		nonce:     authProps.nonce,
		uri:       req.URL.Path,
		algorithm: authProps.algorithm,
		opaque:    authProps.opaque,
		qop:       authProps.qop[0],
		nc:        1,
		userhash:  authProps.userhash,
	}

	return authz.computeFields(username, password, req)
}

// computeFields computes and sets the userhash, cnonce and response attributes
func (authz *Authorization) computeFields(
	username string,
	password string,
	req *http.Request,
) (*Authorization, error) {
	if authz.userhash {
		hashedUsername, err := authz.computeUserhash(username)
		if err != nil {
			return nil, err
		}
		authz.username = hashedUsername
	} else {
		authz.username = username
	}

	cnonce, err := authz.generateCNonce()
	if err != nil {
		return nil, err
	}
	authz.cnonce = cnonce

	response, err := authz.computeResponse(username, password, req)
	if err != nil {
		return nil, err
	}
	authz.response = response

	return authz, nil
}

// generateCNonce generates the client nonce
func (authz *Authorization) generateCNonce() (string, error) {
	b := make([]byte, 8)
	_, err := io.ReadFull(rand.Reader, b)

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", b)[:16], nil
}

// computeUserhash hashes the username: https://tools.ietf.org/html/rfc7616#section-3.4.4
func (authz *Authorization) computeUserhash(username string) (string, error) {
	return computeHash(fmt.Sprintf("%s:%s", username, authz.realm), authz.algorithm)
}

// computeHA1 computes the A1 hash: https://tools.ietf.org/html/rfc7616#section-3.4.2
func (authz *Authorization) computeHA1(username string, password string) (string, error) {
	ha1, err := computeHash(fmt.Sprintf("%s:%s:%s", username, authz.realm, password), authz.algorithm)
	if err != nil {
		return "", err
	}

	if strings.Contains(strings.ToUpper(authz.algorithm), "SESS") {
		return computeHash(fmt.Sprintf("%s:%s:%s", ha1, authz.nonce, authz.cnonce), authz.algorithm)
	}

	return ha1, nil
}

// computeHA2 computes the A2 hash: https://tools.ietf.org/html/rfc7616#section-3.4.3
func (authz *Authorization) computeHA2(method string, uri string, body string) (string, error) {
	if authz.qop == QOP_AUTH_INT {
		bodyHash, err := computeHash(body, authz.algorithm)
		if err != nil {
			return "", nil
		}

		return computeHash(fmt.Sprintf("%s:%s:%s", method, uri, bodyHash), authz.algorithm)
	}

	return computeHash(fmt.Sprintf("%s:%s", method, uri), authz.algorithm)
}

// computeResponse computes the response hash: https://tools.ietf.org/html/rfc7616#section-3.4.1
func (authz *Authorization) computeResponse(username string, password string, req *http.Request) (string, error) {
	ha1, err := authz.computeHA1(username, password)
	if err != nil {
		return "", err
	}

	buf := new(strings.Builder)
	if req.Body != nil {
		_, err = io.Copy(buf, req.Body)
		if err != nil {
			return "", err
		}
	}

	ha2, err := authz.computeHA2(req.Method, req.URL.Path, buf.String())
	if err != nil {
		return "", err
	}

	return computeHash(fmt.Sprintf(
		"%s:%s:%08x:%s:%s:%s",
		ha1,
		authz.nonce,
		authz.nc,
		authz.cnonce,
		authz.qop,
		ha2,
	), authz.algorithm)
}

// String generates the "Authorization" request header field value: https://tools.ietf.org/html/rfc7616#section-3.4
func (authz *Authorization) String() string {
	var buffer bytes.Buffer
	buffer.WriteString("Digest ")

	value := reflect.ValueOf(*authz)
	typeOfS := value.Type()

	for i := 0; i < value.NumField(); i++ {
		fieldName := typeOfS.Field(i).Name

		switch fieldName {
		case "nc":
			buffer.WriteString(fmt.Sprintf("%s=%08x, ", fieldName, value.Field(i).Int()))
		case "userhash":
			buffer.WriteString(fmt.Sprintf("%s=%s, ", fieldName, strconv.FormatBool(value.Field(i).Bool())))
		default:
			if v, ok := typeOfS.Field(i).Tag.Lookup("quoted"); ok && v == "true" {
				buffer.WriteString(fmt.Sprintf("%s=\"%s\", ", fieldName, value.Field(i).String()))
			} else {
				buffer.WriteString(fmt.Sprintf("%s=%s, ", fieldName, value.Field(i).String()))
			}

		}
	}

	return strings.TrimSuffix(buffer.String(), ", ")
}

// computeHash hashes the given str using the specified algorithm
func computeHash(str string, algorithm string) (string, error) {
	algorithm = strings.ToUpper(algorithm)

	var hashMethod hash.Hash
	switch algorithm {
	case MD5, MD5_SESS:
		hashMethod = md5Writer
	case SHA256, SHA256_SESS:
		hashMethod = sha256Writer
	default:
		return "", fmt.Errorf("Algorithm %s not supported", algorithm)
	}

	hashMethod.Reset()
	_, err := io.WriteString(hashMethod, str)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hashMethod.Sum(nil)), nil
}

// Authorize orchestrates the digest authorization process. If authz is not required, an empty string is returned,
// otherwise the "Authorization" header field value is returned
func Authorize(username, password string, req *http.Request, client *http.Client) (string, error) {
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return "", fmt.Errorf("Unexpected HTTP response code received %d instead of 401 Unauthorized. The requested resources might not require authorization", resp.StatusCode)
	}

	if len(resp.Header["Www-Authenticate"]) > 0 {
		authProps := newWwwAuthenticateProps(resp.Header["Www-Authenticate"][0])
		digest, err := createDigestAuthorization(username, password, req, authProps)
		if err != nil {
			return "", err
		}

		return digest.String(), nil
	}

	return "", nil
}
