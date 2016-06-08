// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package test contains functions used for tests.
package test

import (
	"fmt"

	"github.com/stretchr/testify/mock"
)

// ByteArrayArg gets the argument at the specified index. Panics if there is no
// argument, or if the argument is of the wrong type.
func ByteArrayArg(args mock.Arguments, index int) []byte {
	var s []byte
	var ok bool
	if s, ok = args.Get(index).([]byte); !ok {
		panic(fmt.Sprintf("assert: arguments: ByteArrayArg(%d) failed because object wasn't correct type: %v", index, args.Get(index)))
	}
	return s
}
