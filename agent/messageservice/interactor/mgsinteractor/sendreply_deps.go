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
// either express or implied. See the License for the specific language governing`
// permissions and limitations under the License.

// Package mgsinteractor will be responsible for interacting with MGS
package mgsinteractor

import (
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	replytypesutils "github.com/aws/amazon-ssm-agent/agent/messageservice/interactor/mgsinteractor/replytypes"
	"github.com/carlescere/scheduler"
)

// AgentResultLocalStoreData represents the format of data
// that we will store it in the local disk for failed replies
type AgentResultLocalStoreData struct {
	AgentResult contracts.DocumentResult
	ReplyId     string
	RetryNumber int
}

// agentReplyLocalContract represents the contract that we use internally in the send reply code
type agentReplyLocalContract struct {
	documentResult replytypesutils.IReplyType
	backupFile     string
	retryNumber    int
}

// sendReplyProperties represents the sendreply process related properties
type sendReplyProperties struct {
	replyQueueLimit    int
	replyThreadDone    chan struct{}
	reply              chan *agentReplyLocalContract
	replyAckChan       sync.Map
	sendFailedReplyJob *scheduler.Job
	allReplyClosed     chan struct{}
}
