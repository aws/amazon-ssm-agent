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

// Package replytypes will be responsible for handling various replies received from the processor
package replytypes

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/twinj/uuid"
)

type ReplyTypeGenerator func(ctx context.T, res contracts.DocumentResult, replyId uuid.UUID, retryNumber int) IReplyType

var replyTypes map[contracts.ResultType]ReplyTypeGenerator

func init() {
	runCommandReplyFn := NewAgentRunCommandReplyType
	sessionCompleteReplyFn := NewSessionCompleteType
	// In the future, we can register this based on doc type to support different cases
	registerReplyTypes(contracts.RunCommandResult, runCommandReplyFn)
	registerReplyTypes(contracts.SessionResult, sessionCompleteReplyFn) // For now, for all results, we send result with just 1 topic which is session complete
}

// registerReplyTypes used to register the reply types
func registerReplyTypes(name contracts.ResultType, reply ReplyTypeGenerator) {
	if replyTypes == nil {
		replyTypes = make(map[contracts.ResultType]ReplyTypeGenerator)
	}
	replyTypes[name] = reply
}

// GetReplyTypeObject returns the replytype object based on the reply type from the result
func GetReplyTypeObject(ctx context.T, res contracts.DocumentResult, replyId uuid.UUID, retryNumber int) (IReplyType, error) {
	if replyTypes == nil {
		return nil, fmt.Errorf("no reply types found")
	}
	if replyType, ok := replyTypes[res.ResultType]; ok {
		return replyType(ctx, res, replyId, retryNumber), nil
	}
	return nil, fmt.Errorf("the given reply type not found %v", res.ResultType)
}
