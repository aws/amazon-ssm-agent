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

// Package main represents the entry point of the ssm agent updater.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	loginterface "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/log/logger"
	"github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
	"github.com/aws/amazon-ssm-agent/agent/update/processor"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identity2 "github.com/aws/amazon-ssm-agent/common/identity/identity"
	"github.com/nightlyone/lockfile"
)

const (
	defaultLogFileName              = "AmazonSSMAgent-update.txt"
	defaultWaitTimeForAgentToFinish = 3
	errorExitCode                   = 1
	nonErrorExitCode                = 0
)

var (
	updater      processor.T
	log          loginterface.T
	agentContext context.T
)

var (
	update              *bool
	sourceVersion       *string
	sourceLocation      *string
	sourceHash          *string
	targetVersion       *string
	targetLocation      *string
	targetHash          *string
	packageName         *string
	messageID           *string
	stdout              *string
	stderr              *string
	outputKeyPrefix     *string
	outputBucket        *string
	manifestURL         *string
	selfUpdate          *bool
	disableDowngrade    *bool
	upstreamServiceName *string
)

var (
	newAgentIdentity                 = identity2.NewAgentIdentity
	isIdentityRuntimeConfigSupported = updateutil.IsIdentityRuntimeConfigSupported
)

func init() {
	log = ssmlog.GetUpdaterLogger(logger.DefaultLogDir, defaultLogFileName)

	// Load update detail from command line
	update = flag.Bool(updateconstants.UpdateCmd, false, "current Agent Version")
	sourceVersion = flag.String(updateconstants.SourceVersionCmd, "", "current Agent Version")
	sourceLocation = flag.String(updateconstants.SourceLocationCmd, "", "current Agent installer source")

	targetVersion = flag.String(updateconstants.TargetVersionCmd, "", "target Agent Version")

	packageName = flag.String(updateconstants.PackageNameCmd, "", "target Agent Version")
	messageID = flag.String(updateconstants.MessageIDCmd, "", "target Agent Version")
	stdout = flag.String(updateconstants.StdoutFileName, "", "standard output file path")
	stderr = flag.String(updateconstants.StderrFileName, "", "standard error file path")
	outputKeyPrefix = flag.String(updateconstants.OutputKeyPrefixCmd, "", "output key prefix")
	outputBucket = flag.String(updateconstants.OutputBucketNameCmd, "", "output bucket name")

	manifestURL = flag.String(updateconstants.ManifestFileUrlCmd, "", "Manifest file url")

	selfUpdate = flag.Bool(updateconstants.SelfUpdateCmd, false, "SelfUpdate command")

	disableDowngrade = flag.Bool(updateconstants.DisableDowngradeCmd, false, "defines if updater is allowed to downgrade")

	upstreamServiceName = flag.String(updateconstants.UpstreamServiceName, string(contracts.MessageDeliveryService), "defines the upstream messaging service")

	// Legacy flags no longer used, need to be defined or we get this error: flag provided but not defined
	flag.String(updateconstants.TargetLocationCmd, "", "target Agent installer source")
	flag.String(updateconstants.TargetHashCmd, "", "target Agent installer hash")
	flag.String(updateconstants.SourceHashCmd, "", "current Agent installer hash")
}

func main() {
	log.Infof("SSM Agent Updater - %s", version.String())
	os.Exit(updateAgent())
}

func updateAgent() int {
	defer log.Close()
	defer log.Flush()

	flag.Parse()

	// Initialize agent config for agent identity
	appConfig, err := appconfig.Config(true)
	if err != nil {
		log.Warnf("Failed to load agent config: %v", err)
	}
	// Create identity selector
	agentIdentity, err := resolveAgentIdentity(appConfig)
	if err != nil {
		log.Errorf("Failed to assume agent identity: %v", err)
		return errorExitCode
	}

	agentContext = context.Default(log, appConfig, agentIdentity)
	updateUtilRef := updateutil.NewUpdaterUtilWithLoadedDocContent(agentContext, *messageID)
	updateSSMUserShellProperties(log)
	// Create update info
	updateInfo, err := updateinfo.New(agentContext)
	if err != nil {
		log.Errorf("Failed to initialize update info object: %v", err)
		return errorExitCode
	}

	// Sleep 3 seconds to allow agent to finishing up it's work
	time.Sleep(defaultWaitTimeForAgentToFinish * time.Second)

	updater = processor.NewUpdater(agentContext, updateInfo, updateUtilRef)

	// If the updater already owns the lockfile, no harm done
	// If there is no lockfile, the updater will own it
	// If the updater is unable to lock the file, we retry and then fail
	lock, _ := lockfile.New(appconfig.UpdaterPidLockfile)
	err = lock.TryLockExpireWithRetry(updateconstants.UpdateLockFileMinutes)

	if err != nil {
		if err == lockfile.ErrBusy {
			log.Warnf("Failed to lock update lockfile, another update is in progress: %s", err)
			return nonErrorExitCode
		} else {
			log.Warnf("Proceeding update process with new lock. Failed to lock update lockfile: %s", err)
		}
	}

	defer lock.Unlock()

	// Return if update is not present in the command
	if !*update {
		log.Error("incorrect usage (use -update).")
		flag.Usage()
		return nonErrorExitCode
	}

	// Basic Validation
	if len(*manifestURL) == 0 && len(*sourceLocation) == 0 {
		log.Error("must pass either manifest url or source location")
		flag.Usage()
	}
	if len(*sourceVersion) == 0 {
		log.Error("no current version")
		flag.Usage()
	}
	if !*selfUpdate && len(*targetVersion) == 0 {
		log.Error("no target version")
		flag.Usage()
	}

	// Create new UpdateDetail
	updateDetail := &processor.UpdateDetail{
		State:               processor.NotStarted,
		Result:              contracts.ResultStatusInProgress,
		SourceVersion:       *sourceVersion,
		SourceLocation:      *sourceLocation,
		TargetVersion:       *targetVersion,
		StdoutFileName:      *stdout,
		StderrFileName:      *stderr,
		OutputS3KeyPrefix:   *outputKeyPrefix,
		OutputS3BucketName:  *outputBucket,
		PackageName:         *packageName,
		MessageID:           *messageID,
		StartDateTime:       time.Now().UTC(),
		RequiresUninstall:   false,
		ManifestURL:         *manifestURL,
		Manifest:            updatemanifest.New(agentContext, updateInfo, ""),
		SelfUpdate:          *selfUpdate,
		AllowDowngrade:      !*disableDowngrade,
		UpstreamServiceName: *upstreamServiceName,
	}

	updateDetail.UpdateRoot, err = updateutil.ResolveUpdateRoot(updateDetail.SourceVersion)
	if err != nil {
		log.Errorf("Failed to resolve update root: %v", err)
		return nonErrorExitCode
	}

	log.Infof("Update root is: %v", updateDetail.UpdateRoot)

	// Initialize update detail with plugin info
	err = updater.InitializeUpdate(log, updateDetail)
	if err != nil {
		log.Errorf(err.Error())
		return nonErrorExitCode
	}

	// Recover updater if panic occurs and fail the updater
	defer recoverUpdaterFromPanic(updateDetail)

	// Start or resume update
	if err = updater.StartOrResumeUpdate(log, updateDetail); err != nil { // We do not send any error above this to ICS/MGS except panic message
		// Rolled back, but service cannot start, Update failed.
		updater.Failed(updateDetail, log, updateconstants.ErrorUnexpected, err.Error(), false)
	} else {
		log.Infof(updateDetail.StandardOut)
	}

	return nonErrorExitCode
}

func resolveAgentIdentity(appConfig appconfig.SsmagentConfig) (identity.IAgentIdentity, error) {
	var selector identity2.IAgentIdentitySelector
	var agentIdentity identity.IAgentIdentity
	var err error
	// To support downgrades and rollbacks, we want to make sure that the source version supports runtime config
	if isIdentityRuntimeConfigSupported(*sourceVersion) {
		selector = identity2.NewRuntimeConfigIdentitySelector(log)
		agentIdentity, err = newAgentIdentity(log, &appConfig, selector)

		// If success, return the identity
		if err == nil {
			log.Debugf("Using identity from runtime config")
			return agentIdentity, nil
		}
	}

	// If not able to resolve agent identity with runtime config or source version
	// does not support runtimeconfig, fallback to default identity selector
	selector = identity2.NewDefaultAgentIdentitySelector(log)
	agentIdentity, err = newAgentIdentity(log, &appConfig, selector)
	if err != nil {
		return nil, err
	}

	return agentIdentity, nil
}

// recoverUpdaterFromPanic recovers updater if panic occurs and fails the updater
func recoverUpdaterFromPanic(updateDetail *processor.UpdateDetail) {
	// recover in case the updater panics
	if err := recover(); err != nil {
		agentContext.Log().Errorf("recovered from panic for updater %v!", err)
		agentContext.Log().Errorf("Stacktrace:\n%s", debug.Stack())
		updater.Failed(updateDetail, agentContext.Log(), updateconstants.ErrorUnexpectedThroughPanic, fmt.Sprintf("%v", err), false)
	}
}
