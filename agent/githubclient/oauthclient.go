// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package githubclient contains methods for interacting with git
package githubclient

import (
	gitcontext "golang.org/x/net/context"

	"net/http"

	"golang.org/x/oauth2"
)

// OAuthClient is a wrapper around github.Client. This is done for mocking
type OAuthClient struct{}

// IOAuthClient is an interface for oauth access of Github
type IOAuthClient interface {
	GetGithubOauthClient(token string) *http.Client
}

// GetGithubOauthClient returns the http client using oauth access tokens
// implementation of this has been taken from https://github.com/google/go-github#authentication
func (git OAuthClient) GetGithubOauthClient(token string) *http.Client {
	ctx := gitcontext.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	return oauth2.NewClient(ctx, ts)
}
