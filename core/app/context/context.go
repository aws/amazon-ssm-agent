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

// Package context contains context details needed by the app to execute
package context

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
)

// ICoreAgentContext defines a type that carries context specific data such as the logger.
type ICoreAgentContext interface {
	Log() log.T
	AppConfig() *appconfig.SsmagentConfig
	Identity() identity.IAgentIdentity
	With(context string) ICoreAgentContext
}

// CoreAgentContext defines a type that carries context specific data such as the logger.
type CoreAgentContext struct {
	context   []string
	log       log.T
	appConfig *appconfig.SsmagentConfig
	identity  identity.IAgentIdentity
}

// With updates the contextSlice that changes the log prefix
func (c *CoreAgentContext) With(logContext string) ICoreAgentContext {
	contextSlice := append(c.context, logContext)
	newContext := &CoreAgentContext{
		context:   contextSlice,
		log:       c.log.WithContext(contextSlice...),
		appConfig: c.appConfig,
		identity:  c.identity,
	}
	return newContext
}

// Log returns the log
func (c *CoreAgentContext) Log() log.T {
	return c.log
}

// AppConfig returns app config
func (c *CoreAgentContext) AppConfig() *appconfig.SsmagentConfig {
	return c.appConfig
}

// Identity returns identity object
func (c *CoreAgentContext) Identity() identity.IAgentIdentity {
	return c.identity
}

// NewCoreAgentContext creates and returns a new core agent context
func NewCoreAgentContext(logger log.T, ssmAppconfig *appconfig.SsmagentConfig, agentIdentity identity.IAgentIdentity) (ICoreAgentContext, error) {
	coreContext := &CoreAgentContext{
		appConfig: ssmAppconfig,
		log:       logger,
		identity:  agentIdentity,
	}
	return coreContext, nil
}
