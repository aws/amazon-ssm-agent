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
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateprecondition"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updates3util"
)

// T represents the interface for agent update
type T interface {
	// StartOrResumeUpdate starts/resumes update.
	StartOrResumeUpdate(log log.T, updateDetail *UpdateDetail) (err error)

	// InitializeUpdate initializes update
	InitializeUpdate(log log.T, detail *UpdateDetail) (err error)

	// Failed sets update to failed with error messages
	Failed(updateDetail *UpdateDetail, log log.T, code updateconstants.ErrorCode, errMessage string, noRollbackMessage bool) (err error)
}

type initPrep func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error)
type update func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error)
type verify func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error)
type rollback func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error)
type uninstall func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error)
type install func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error)
type download func(mgr *updateManager, log log.T, downloadInput artifact.DownloadInput, updateDetail *UpdateDetail, version string) (err error)
type clean func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error)
type runTests func(context context.T, stage testCommon.TestStage, timeOutSeconds int) (testOutput string)
type finalize func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error)

type updateManager struct {
	Context             context.T
	Info                updateinfo.T
	util                updateutil.T
	S3util              updates3util.T
	preconditions       []updateprecondition.T
	svc                 Service
	ctxMgr              ContextMgr
	initManifest        initPrep
	initSelfUpdate      initPrep
	determineTarget     initPrep
	validateUpdateParam initPrep
	populateUrlHash     initPrep
	downloadPackages    initPrep
	update              update
	verify              verify
	rollback            rollback
	uninstall           uninstall
	install             install
	download            download
	clean               clean
	runTests            runTests
	finalize            finalize
	subStatus           string // Values currently being used - downgrade, InstallRollback, VerificationRollback.
}

// Updater contains logic for performing agent update
type Updater struct {
	mgr *updateManager
}
