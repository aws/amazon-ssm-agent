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

// Package context defines a type that carries context specific data such as the logger.
// Inspired by Google's http://godoc.org/golang.org/x/net/context
package context

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
)

// T transfers context specific data across different execution boundaries.
// Instead of adding the context to specific structs, we pass Context as the first
// parameter to the methods themselves.
type T interface {
	Log() log.T
	AppConfig() appconfig.SsmagentConfig
	With(context string) T
	CurrentContext() []string
	AppConstants() *appconfig.AppConstants
	Identity() identity.IAgentIdentity
}

// Default returns an empty context that use the default logger and appconfig.
func Default(logger log.T, ssmAppconfig appconfig.SsmagentConfig, agentIdentity identity.IAgentIdentity, contextList ...string) T {
	// Loading the maximum & minimum frequency minutes for healthcheck
	appconst := appconfig.AppConstants{
		MinHealthFrequencyMinutes: appconfig.DefaultSsmHealthFrequencyMinutesMin,
		MaxHealthFrequencyMinutes: appconfig.DefaultSsmHealthFrequencyMinutesMax,
	}
	return &defaultContext{context: contextList, log: logger.WithContext(contextList...), appconfig: ssmAppconfig, appconst: appconst, identity: agentIdentity}
}

type defaultContext struct {
	context   []string
	log       log.T
	appconfig appconfig.SsmagentConfig
	appconst  appconfig.AppConstants
	identity  identity.IAgentIdentity
}

func (c *defaultContext) With(logContext string) T {
	contextSlice := append(c.context, logContext)
	newContext := &defaultContext{
		context:   contextSlice,
		log:       c.log.WithContext(contextSlice...),
		appconfig: c.appconfig,
		appconst:  c.appconst,
		identity:  c.identity,
	}
	return newContext
}

func (c *defaultContext) Log() log.T {
	return c.log
}

func (c *defaultContext) AppConfig() appconfig.SsmagentConfig {
	return c.appconfig
}

func (c *defaultContext) CurrentContext() []string {
	return c.context
}

func (c *defaultContext) AppConstants() *appconfig.AppConstants {
	return &c.appconst
}

func (c *defaultContext) Identity() identity.IAgentIdentity {
	return c.identity
}
