// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package credentialproviders

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go-v2/credentials/endpointcreds"
)

// Environment variables that select the container credentials endpoint.
const (
	envContainerFullURI  = "AWS_CONTAINER_CREDENTIALS_FULL_URI"
	envContainerRelURI   = "AWS_CONTAINER_CREDENTIALS_RELATIVE_URI"
	envContainerAuthTok  = "AWS_CONTAINER_AUTHORIZATION_TOKEN"
	envContainerAuthFile = "AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE"

	// ecsContainerCredentialsHost is the fixed loopback host that ECS uses
	// to serve task role credentials over the RELATIVE_URI path.
	ecsContainerCredentialsHost = "http://169.254.170.2"

	// ecsCredsExpiryWindow is the expiry window applied to container
	// credentials cached via aws.NewCredentialsCache.
	ecsCredsExpiryWindow = 5 * time.Minute
)

// Loopback / link-local IPs allowed for the FULL_URI branch when using
// plain http.
var (
	ecsContainerIPv4 = net.IPv4(169, 254, 170, 2)  // ECS
	eksContainerIPv4 = net.IPv4(169, 254, 170, 23) // EKS Pod Identity (IPv4)
	eksContainerIPv6 = net.ParseIP("fd00:ec2::23") // EKS Pod Identity (IPv6)
)

// lookupHost is a hook so tests can stub DNS resolution when validating
// AWS_CONTAINER_CREDENTIALS_FULL_URI.
var lookupHost = net.LookupHost

// noFailRefreshExtension wraps an aws.CredentialsProvider so that
// aws.CredentialsCache does NOT invoke HandleFailToRefresh on it — even if
// the inner provider implements that hook (e.g. ec2rolecreds.Provider, which
// extends the Expires time by a random 5–15 minutes when Retrieve fails).
//
// With the default extension in play, a detached IAM instance profile keeps
// returning stale credentials to the cache for up to ~1 hour, so role
// revocation isn't promptly effective on the caller. Omitting the
// HandleFailToRefresh method causes CredentialsCache's interface check to
// fall back to returning the underlying error immediately.
//
// Callers should apply this before handing an ec2rolecreds provider (or any
// long-lived provider that implements HandleFailToRefresh) to
// aws.NewCredentialsCache.
type noFailRefreshExtension struct {
	inner aws.CredentialsProvider
}

func (p *noFailRefreshExtension) Retrieve(ctx context.Context) (aws.Credentials, error) {
	return p.inner.Retrieve(ctx)
}

func GetRemoteCreds() aws.Credentials {
	appCreds := aws.NewCredentialsCache(&noFailRefreshExtension{ec2rolecreds.New()})
	value, err := appCreds.Retrieve(context.TODO())
	if err != nil {
		// handle error
	}

	return value
}

func GetRemoteCredsProvider() aws.CredentialsProvider {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	provider := cfg.Credentials
	return provider
}

func GetDefaultCredsProvider() aws.CredentialsProvider {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	return cfg.Credentials
}

// GetECSCredsProvider returns credentials for a container / EC2 environment
// using the following precedence:
//
//  1. If AWS_CONTAINER_CREDENTIALS_FULL_URI is set → HTTP endpoint provider
//     against that URI. For http:// URIs the host is validated against the
//     ECS/EKS allowlist. https:// URIs are permitted for any host — TLS is
//     the trust anchor.
//  2. Else if AWS_CONTAINER_CREDENTIALS_RELATIVE_URI is set → HTTP endpoint
//     provider against http://169.254.170.2<relative>.
//  3. Else → fall through to the default credential chain, which resolves EC2
//     IMDS. LoadDefaultConfig is not consulted when a container URI is set
//     because the SDK's chain has other resolvers ahead of the container path
//     and could return an unrelated (invalid) source instead of the task role.
//
// AWS_CONTAINER_AUTHORIZATION_TOKEN and AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE
// are honored on both HTTP branches. Credentials are wrapped in
// aws.NewCredentialsCache with a 5-minute expiry window.
func GetECSCredsProvider() aws.CredentialsProvider {
	if fullURI := os.Getenv(envContainerFullURI); fullURI != "" {
		if err := validateFullContainerURI(fullURI); err != nil {
			return errCredsProvider(fmt.Errorf(
				"invalid %s: %w", envContainerFullURI, err))
		}
		return newHTTPContainerCredsProvider(fullURI)
	}

	if relURI := os.Getenv(envContainerRelURI); relURI != "" {
		return newHTTPContainerCredsProvider(ecsContainerCredentialsHost + relURI)
	}

	// No container credentials env — let the SDK default chain resolve
	// (typically EC2 IMDS on the underlying instance).
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		// Wrap in noFailRefreshExtension so a failed IMDS Retrieve (e.g. the
		// underlying host's instance profile got detached) surfaces the error
		// immediately instead of the ec2rolecreds provider extending its
		// stale credentials by 5–15 minutes via HandleFailToRefresh.
		return aws.NewCredentialsCache(&noFailRefreshExtension{ec2rolecreds.New()})
	}
	return cfg.Credentials
}

// newHTTPContainerCredsProvider builds an endpointcreds provider for the
// given URI, attaching AWS_CONTAINER_AUTHORIZATION_TOKEN[_FILE] if present,
// and wraps it in a CredentialsCache with a 5-minute expiry window.
func newHTTPContainerCredsProvider(uri string) aws.CredentialsProvider {
	optFns := []func(*endpointcreds.Options){}

	if tok := os.Getenv(envContainerAuthTok); tok != "" {
		optFns = append(optFns, func(o *endpointcreds.Options) {
			o.AuthorizationToken = tok
		})
	}

	if authFile := os.Getenv(envContainerAuthFile); authFile != "" {
		optFns = append(optFns, func(o *endpointcreds.Options) {
			o.AuthorizationTokenProvider = endpointcreds.TokenProviderFunc(func() (string, error) {
				contents, err := os.ReadFile(authFile)
				if err != nil {
					return "", fmt.Errorf(
						"read %s %q: %w", envContainerAuthFile, authFile, err)
				}
				return string(contents), nil
			})
		})
	}

	return aws.NewCredentialsCache(
		endpointcreds.New(uri, optFns...),
		func(o *aws.CredentialsCacheOptions) {
			o.ExpiryWindow = ecsCredsExpiryWindow
		},
	)
}

// validateFullContainerURI enforces that http:// endpoints resolve to
// loopback or one of the well-known ECS/EKS link-local IPs. https://
// endpoints are always allowed (TLS + cert is the trust anchor).
func validateFullContainerURI(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse URI: %w", err)
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme != "http" {
		return fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("empty host")
	}

	// Literal IP host — check directly.
	if ip := net.ParseIP(host); ip != nil {
		if isContainerCredsHostIP(ip) {
			return nil
		}
		return fmt.Errorf(
			"host %q not allowed for http; only loopback, %s, %s and %s are permitted",
			host, ecsContainerIPv4, eksContainerIPv4, eksContainerIPv6)
	}

	// Hostname — resolve and require every resulting IP to be allowed.
	addrs, err := lookupHost(host)
	if err != nil {
		return fmt.Errorf("resolve host %q: %w", host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("host %q resolved to no addresses", host)
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil || !isContainerCredsHostIP(ip) {
			return fmt.Errorf(
				"host %q resolves to disallowed address %q", host, addr)
		}
	}
	return nil
}

func isContainerCredsHostIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.Equal(ecsContainerIPv4) ||
		ip.Equal(eksContainerIPv4) ||
		ip.Equal(eksContainerIPv6)
}

// errCredsProvider returns a CredentialsProvider that always fails with err.
// Used when AWS_CONTAINER_CREDENTIALS_FULL_URI is set but invalid, so the
// error surfaces to the caller instead of falling through to IMDS (which
// would sign with the wrong identity).
func errCredsProvider(err error) aws.CredentialsProvider {
	return aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{}, err
	})
}
