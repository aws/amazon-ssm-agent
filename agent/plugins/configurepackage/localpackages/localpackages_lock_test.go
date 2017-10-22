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

package localpackages

import (
	"os"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/fileutil/filelock"
	"github.com/stretchr/testify/assert"
)

var fileLocker = filelock.NewFileLocker()

// var fileLocker = &filelock.FileLockerNoop{}

func TestPackageLock(t *testing.T) {
	os.Remove("lockpath")

	// lock Foo for Install
	err := lockPackage(fileLocker, "lockpath-Foo", "Foo", "Install")
	assert.Nil(t, err)
	defer unlockPackage(fileLocker, "lockpath-Foo", "Foo")

	// shouldn't be able to lock Foo, even for a different action
	err = lockPackage(fileLocker, "lockpath-Foo", "Foo", "Uninstall")
	assert.NotNil(t, err)

	// lock and unlock Bar (with defer)
	err = lockAndUnlock("lockpath-Bar", "Bar")
	assert.Nil(t, err)

	// should be able to lock and then unlock Bar
	err = lockPackage(fileLocker, "lockpath-Bar", "Bar", "Uninstall")
	assert.Nil(t, err)
	unlockPackage(fileLocker, "lockpath-Bar", "Bar")

	// should be able to lock Bar
	err = lockPackage(fileLocker, "lockpath-Bar", "Bar", "Uninstall")
	assert.Nil(t, err)
	defer unlockPackage(fileLocker, "lockpath-Bar", "Bar")

	// lock in a goroutine with a 10ms sleep
	errorChan := make(chan error)
	go lockAndUnlockGo("lockpath-Foobar", "Foobar", errorChan)
	err = <-errorChan // wait until the goroutine has acquired the lock
	assert.Nil(t, err)
	err = lockPackage(fileLocker, "lockpath-Foobar", "Foobar", "Install")
	errorChan <- err // signal the goroutine to exit
	assert.NotNil(t, err)
}

func lockAndUnlockGo(lockpath string, packageName string, channel chan error) {
	err := lockPackage(fileLocker, lockpath, packageName, "Install")
	channel <- err
	_ = <-channel
	if err == nil {
		defer unlockPackage(fileLocker, lockpath, packageName)
	}
	return
}

func lockAndUnlock(lockpath string, packageName string) (err error) {
	if err = lockPackage(fileLocker, lockpath, packageName, "Install"); err != nil {
		return
	}
	defer unlockPackage(fileLocker, lockpath, packageName)
	return
}

/*
func TestRunParallelSamePackage(t *testing.T) {
	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	managerMockFirst := ConfigPackageSuccessMock("/foo", "Wait1.0.0", "", contracts.ResultStatusSuccess, contracts.ResultStatusSuccess, contracts.ResultStatusSuccess)
	managerMockSecond := ConfigPackageSuccessMock("/foo", "1.0.0", "", contracts.ResultStatusSuccess, contracts.ResultStatusSuccess, contracts.ResultStatusSuccess)

	var outputFirst contracts.PluginOutput
	var outputSecond contracts.PluginOutput
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		outputFirst = runConfigurePackage(plugin, contextMock, managerMockFirst, pluginInformation)
	}()
	// wait until first call is at getVersionToInstall
	_ = <-managerMockFirst.waitChan
	// start second call
	outputSecond = runConfigurePackage(plugin, contextMock, managerMockSecond, pluginInformation)
	// after second call completes, allow first call to continue
	managerMockFirst.waitChan <- true
	// wait until first call is complete
	wg.Wait()

	assert.Equal(t, outputFirst.ExitCode, 0)
	assert.Equal(t, outputSecond.ExitCode, 1)
	assert.True(t, strings.Contains(outputSecond.Stderr, `Package "PVDriver" is already in the process of action "Install"`))
}
*/
/*
func (configMock *MockedConfigurePackageManager) getVersionToInstall(context context.T,
	input *ConfigurePackagePluginInput) (version string, installedVersion string, installState localpackages.InstallState, err error) {
	args := configMock.Called(input)
	ver := args.String(0)
	if strings.HasPrefix(ver, "Wait") {
		configMock.waitChan <- true
		_ = <-configMock.waitChan
		ver = strings.TrimLeft(ver, "Wait")
	}
	return ver, args.String(1), args.Get(2).(localpackages.InstallState), args.Error(3)
}

func (configMock *MockedConfigurePackageManager) getVersionToUninstall(context context.T,
	input *ConfigurePackagePluginInput) (version string, err error) {
	args := configMock.Called(input)
	ver := args.String(0)
	if strings.HasPrefix(ver, "Wait") {
		configMock.waitChan <- true
		_ = <-configMock.waitChan
		ver = strings.TrimLeft(ver, "Wait")
	}
	return ver, args.Error(1)
}
*/
