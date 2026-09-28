// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package util contains helper function common for ssm service
package util

import (
	cont "context"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/aws/aws-sdk-go-v2/aws/retry"

	"github.com/aws/amazon-ssm-agent/agent/sdkutil/retryer"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
	"github.com/aws/aws-sdk-go-v2/aws"
	//"github.com/aws/smithy-go/logging"
)

// AwsConfig builds an aws.Config for the given service/region using the agent's
// custom HTTP transport (proxy, CA bundle, TLS downgrade protection).
//
// The HTTPClient set here is propagated into every credential-chain sub-client
// that LoadDefaultConfig may construct (STS AssumeRole, SSO, IMDS role), so
// those flows inherit the same proxy / custom CA / TLS settings without
// further wiring. credential_process is the only credential source that does
// not go through this transport, since it spawns a subprocess rather than
// making HTTP calls.
func AwsConfig(logger log.T, appConfig appconfig.SsmagentConfig, service, region string) aws.Config {
	endpointHelper := endpoint.NewEndpointHelper(logger, appConfig)

	svcEndpoint := endpointHelper.GetServiceEndpoint(service, region)

	// Apply service-specific endpoint override from appConfig so that it is
	// baked into LoadOptions before the SDK caches ConfigSources. Otherwise a
	// post-hoc mutation of cfg.BaseEndpoint gets silently overwritten by
	// ssm.NewFromConfig's resolveBaseEndpoint, which re-reads LoadOptions via
	// ResolveServiceBaseEndpoint and returns the originally baked endpoint.
	if service == "ssm" && appConfig.Ssm.Endpoint != "" {
		svcEndpoint = appConfig.Ssm.Endpoint
	}

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
		config.WithRetryer(newRetryer),
		config.WithHTTPClient(&http.Client{
			Transport:     network.GetDefaultTransport(logger, appConfig),
			CheckRedirect: network.DisableHTTPDowngrade,
			Timeout:       60 * time.Second,
		}),
		// Disable SDK's env-based dualstack resolution. The agent manages dualstack
		// endpoints itself via appConfig.Agent.UseDualStackEndpoint and builds the
		// correct endpoint URL in endpointHelper. Allowing the SDK to also read
		// AWS_USE_DUALSTACK_ENDPOINT from the environment conflicts with BaseEndpoint
		// and causes "Invalid Configuration: Dualstack and custom endpoint are not supported".
		config.WithUseDualStackEndpoint(aws.DualStackEndpointStateDisabled),
		// Same for FIPS — agent manages endpoints itself, AWS_USE_FIPS_ENDPOINT from
		// env would conflict with BaseEndpoint.
		config.WithUseFIPSEndpoint(aws.FIPSEndpointStateDisabled),
	}
	// Only set BaseEndpoint when the helper produced one. Setting it to an
	// empty value would short-circuit the SDK's ruleset resolver and cause
	// it to hard-error on the empty BaseEndpoint.
	if svcEndpoint != "" {
		opts = append(opts, config.WithBaseEndpoint(svcEndpoint))
	}
	conf, err := config.LoadDefaultConfig(cont.TODO(), opts...)
	if err != nil {
		logger.Errorf("Failed to load AWS config for service %s: %v. Falling back to manual config.", service, err)
		// Fall back to a manually constructed config so proxy/CA/transport are not lost
		fallback := aws.Config{
			Region:  region,
			Retryer: newRetryer,
			HTTPClient: &http.Client{
				Transport:     network.GetDefaultTransport(logger, appConfig),
				CheckRedirect: network.DisableHTTPDowngrade,
				Timeout:       60 * time.Second,
			},
		}
		if svcEndpoint != "" {
			fallback.BaseEndpoint = &svcEndpoint
		}
		return fallback
	}

	return conf
}

// AwsConfigWithEndpoint is like AwsConfig but allows an explicit endpoint override.
//
// The HTTPClient set here is inherited by every credential chain sub-client
// that LoadDefaultConfig may construct (STS AssumeRole, SSO, IMDS role), so
// proxy / custom CA / TLS settings apply uniformly across service calls and
// credential resolution.
func AwsConfigWithEndpoint(logger log.T, appConfig appconfig.SsmagentConfig, service, region string, endpoint *string) aws.Config {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
		config.WithRetryer(newRetryer),
		config.WithHTTPClient(&http.Client{
			Transport:     network.GetDefaultTransport(logger, appConfig),
			CheckRedirect: network.DisableHTTPDowngrade,
			Timeout:       60 * time.Second,
		}),
		// Disable SDK's env-based dualstack resolution. The agent manages dualstack
		// endpoints itself via appConfig.Agent.UseDualStackEndpoint and builds the
		// correct endpoint URL in endpointHelper. Allowing the SDK to also read
		// AWS_USE_DUALSTACK_ENDPOINT from the environment conflicts with BaseEndpoint
		// and causes "Invalid Configuration: Dualstack and custom endpoint are not supported".
		config.WithUseDualStackEndpoint(aws.DualStackEndpointStateDisabled),
		// Same for FIPS — agent manages endpoints itself, AWS_USE_FIPS_ENDPOINT from
		// env would conflict with BaseEndpoint.
		config.WithUseFIPSEndpoint(aws.FIPSEndpointStateDisabled),
	}
	if endpoint != nil && *endpoint != "" {
		opts = append(opts, config.WithBaseEndpoint(*endpoint))
	}
	conf, err := config.LoadDefaultConfig(cont.TODO(), opts...)
	if err != nil {
		if logger != nil {
			logger.Errorf("Failed to load AWS config: %v. Falling back to manual config.", err)
		}
		fallback := aws.Config{
			Region:  region,
			Retryer: newRetryer,
			HTTPClient: &http.Client{
				Transport:     network.GetDefaultTransport(logger, appConfig),
				CheckRedirect: network.DisableHTTPDowngrade,
				Timeout:       60 * time.Second,
			},
		}
		if endpoint != nil && *endpoint != "" {
			fallback.BaseEndpoint = endpoint
		}
		return fallback
	}
	return conf
}

// sharedRetryer is the single SsmRetryer instance handed out by newRetryer.
// aws-sdk-go v2 accepts a func() aws.Retryer factory, but retry.Standard's
// token-bucket rate limiter and quota accounting are per-instance, so a
// fresh retryer per client would give each service call its own 500-token
// bucket. retry.Standard is safe to share across concurrent operations
// (internal state uses atomic accounting), so aliasing a single instance
// here keeps the quota shared across all SDK clients.
var sharedRetryer aws.Retryer = &retryer.SsmRetryer{
	Standard: retry.NewStandard(func(so *retry.StandardOptions) {
		so.MaxAttempts = 4
	}),
}

var newRetryer = func() aws.Retryer {
	return sharedRetryer
}

var sleepDelay = func(d time.Duration) {
	time.Sleep(d)
}
