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

// package filewatcherbasedipc is used to establish IPC between master and workers using files.
//
//go:build darwin || freebsd || linux || netbsd || openbsd
// +build darwin freebsd linux netbsd openbsd

package filewatcherbasedipc

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
)

// cleanUpWatcher removes and closes the file watchers added in this file
func (ch *fileWatcherChannel) cleanUpWatcher(completedWatcherCleanup chan bool, log log.T) {
	defer func() {
		completedWatcherCleanup <- true
		if msg := recover(); msg != nil {
			log.Errorf("file watcher remove/close panics: %v", msg)
		}
		log.Infof("channel %v closed", ch.path)
	}()
	//make sure the file watcher closed as well as the watch list is removed, otherwise can cause leak in ubuntu kernel
	//TODO: Should test various platforms by removing the below Remove() as it is being called inside Close() again. Following the previous developer for now based on the above comment.
	if removeError := ch.watcher.Remove(ch.path); removeError != nil {
		log.Warnf("file watcher remove error: %v", removeError)
	}
	if closeError := ch.watcher.Close(); closeError != nil {
		log.Warnf("file watcher close error: %v", closeError)
	}
}
