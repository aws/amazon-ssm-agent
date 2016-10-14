// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package configurecomponent implements the ConfigureComponent plugin.
package configurecomponent

import (
	"errors"
	"fmt"
	"sync"
)

// Prevent multiple actions for the same component at the same time
var lockComponentAction = &sync.Mutex{}
var mapComponentAction = make(map[string]string)

func lockComponent(component string, action string) error {
	lockComponentAction.Lock()
	defer lockComponentAction.Unlock()
	if val, ok := mapComponentAction[component]; ok {
		return errors.New(fmt.Sprintf("Component {%v} is already in the process of action {%v}", component, val))
	}
	mapComponentAction[component] = action

	return nil
}

func unlockComponent(component string) {
	lockComponentAction.Lock()
	defer lockComponentAction.Unlock()
	if _, ok := mapComponentAction[component]; ok {
		delete(mapComponentAction, component)
	}
}
