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

// Package installer implements interfaces for installing and uninstalling packages
package installer

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
)

// Installer is used to install, uninstall, or upgrade a package that exists in the local repository.
type Installer interface {
	Install(context context.T) *contracts.PluginOutput
	Uninstall(context context.T) *contracts.PluginOutput
	Validate(context context.T) *contracts.PluginOutput // TODO:MF consider whether we can remove validate in V1 - I think it depends on having truly idempotent installers for anything that reboots
	PackageName() string
	Version() string
}
