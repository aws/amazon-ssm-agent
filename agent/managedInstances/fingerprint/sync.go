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

// Package fingerprint contains functions that helps identify an instance
// sync contains funcs required to perform sync
package fingerprint

import (
	"sync"
)

var (
	loaded = false
	lock   sync.RWMutex
)

func isLoaded() bool {
	lock.RLock()
	defer lock.RUnlock()
	return loaded
}

func setLoaded(value bool) {
	lock.Lock()
	defer lock.Unlock()
	loaded = value
}
