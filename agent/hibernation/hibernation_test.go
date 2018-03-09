// // Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// //
// // Licensed under the Apache License, Version 2.0 (the "License"). You may not
// // use this file except in compliance with the License. A copy of the
// // License is located at
// //
// // http://aws.amazon.com/apache2.0/
// //
// // or in the "license" file accompanying this file. This file is distributed
// // on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// // either express or implied. See the License for the specific language governing
// // permissions and limitations under the License.

// // Package hibernation is responsible for the agent in hibernate mode.
// // It depends on health pings in an exponential backoff to check if the agent needs
// // to move to active mode.
package hibernation

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/health"
)

func TestHibernation_ExecuteHibernation_AgentTurnsActive(t *testing.T) {
	ctx := context.NewMockDefault()
	healthMock := health.NewHealthCheck(ctx)

	hibernate := NewHibernateMode(healthMock, ctx)
	hibernate.scheduleHealthPing = fakeScheduler
	for i := 0; i < 4; i++ {
		modeChan <- health.Passive
	}
	var status health.AgentState
	go func(h *Hibernate) {
		status = ExecuteHibernation(h)
		assert.Equal(t, health.Active, status)
	}(hibernate)
	modeChan <- health.Active
}

func fakeScheduler(int, *Hibernate) {
	//Do nothing
}
