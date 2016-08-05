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

// Package processor contains the methods for update ssm agent.
// It also provides methods for sendReply and updateInstanceInfo
package processor

import (
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

// T represents the interface for agent update
type T interface {
	// StartOrResumeUpdate starts/resumes update.
	StartOrResumeUpdate(log log.T, context *UpdateContext) (err error)

	// InitializeUpdate initializes update, creates update context
	InitializeUpdate(log log.T, detail *UpdateDetail) (context *UpdateContext, err error)

	// Failed sets update to failed with error messages
	Failed(context *UpdateContext, log log.T, code updateutil.ErrorCode, errMessage string, noRollbackMessage bool) (err error)
}

type prepare func(mgr *updateManager, log log.T, context *UpdateContext) (err error)
type update func(mgr *updateManager, log log.T, context *UpdateContext) (err error)
type verify func(mgr *updateManager, log log.T, context *UpdateContext, isRollback bool) (err error)
type rollback func(mgr *updateManager, log log.T, context *UpdateContext) (err error)
type uninstall func(mgr *updateManager, log log.T, version string, context *UpdateContext) (err error)
type install func(mgr *updateManager, log log.T, version string, context *UpdateContext) (err error)
type download func(mgr *updateManager, log log.T, downloadInput artifact.DownloadInput, context *UpdateContext, version string) (err error)

type updateManager struct {
	util      updateutil.T
	svc       Service
	ctxMgr    ContextMgr
	prepare   prepare
	update    update
	verify    verify
	rollback  rollback
	uninstall uninstall
	install   install
	download  download
}

// Updater contains logic for performing agent update
type Updater struct {
	mgr *updateManager
}
