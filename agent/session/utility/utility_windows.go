// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
//
// +build windows

// utility package implements all the shared methods between clients.
package utility

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	// Success code for NetUser function calls
	nerrSuccess = 0
	// Nil is a zero value for pointer types
	nilPointerValue = 0
	// User level privilege
	userPrivUser = 1
	// Sever name for local machine
	serverNameLocalMachine = 0
	// Indicates that the logon script is executed
	ufScript = 1
	// Indicates that the user's account is disabled.
	USER_UF_ACCOUNTDISABLE = 2
	// Indicates running state of a service
	serviceRunning = 4
	// Active Directory Domain Controller service name
	addsServiceName = "NTDS"

	// Information level for domain and name of the new local group member.
	levelForLocalGroupMembersInfo3 = 3
	// Information level for password data
	levelForUserInfo1003 = 1003
	// Information level for user account attributes
	levelForUserInfo1008 = 1008
	// Level for fetching USER_INFO_1 structure data
	levelForUserInfo1 = 1

	// Windows error code for user not found
	errCodeForUserNotFound = 2221
	// Winows error code for user account already exists
	errCodeForUserAlreadyExists = 2224
	// Windows error code for account name already a member of the group
	errCodeForUserAlreadyGroupMember = 1378

	logon32LogonNetwork    = uintptr(3)
	logon32ProviderDefault = uintptr(0)
)

type USER_INFO_1003 struct {
	Usri1003_password *uint16
}

type USER_INFO_1008 struct {
	Usri1008_flags uint32
}

type USER_INFO_1 struct {
	Usri1_name         *uint16
	Usri1_password     *uint16
	Usri1_password_age uint32
	Usri1_priv         uint32
	Usri1_home_dir     *uint16
	Usri1_comment      *uint16
	Usri1_flags        uint32
	Usri1_script_path  *uint16
}

type USER1_FLAGS struct {
	UF_ACCOUNTDISABLE *uint16
}

type LOCALGROUP_MEMBERS_INFO_3 struct {
	Lgrmi3_domainandname *uint16
}

var (
	modNetapi32             = syscall.NewLazyDLL("netapi32.dll")
	netUserSetInfo          = modNetapi32.NewProc("NetUserSetInfo")
	netUserGetInfo          = modNetapi32.NewProc("NetUserGetInfo")
	netUserAdd              = modNetapi32.NewProc("NetUserAdd")
	netApiBufferFree        = modNetapi32.NewProc("NetApiBufferFree")
	netLocalGroupAddMembers = modNetapi32.NewProc("NetLocalGroupAddMembers")
	advapi32                = syscall.NewLazyDLL("advapi32.dll")
	userenv                 = syscall.NewLazyDLL("userenv.dll")
	logonProc               = advapi32.NewProc("LogonUserW")
	loadUserProfileW        = userenv.NewProc("LoadUserProfileW")
	unloadUserProfile       = userenv.NewProc("UnloadUserProfile")
	getUserProfileDirectory = userenv.NewProc("GetUserProfileDirectoryW")
	duplicateToken          = advapi32.NewProc("DuplicateToken")
	impersonateProc         = advapi32.NewProc("ImpersonateLoggedOnUser")
	revertSelfProc          = advapi32.NewProc("RevertToSelf")
)

type ProfileInfo struct {
	Size        uint32
	Flags       uint32
	UserName    *uint16
	ProfilePath *uint16
	DefaultPath *uint16
	ServerName  *uint16
	PolicyPath  *uint16
	Profile     syscall.Handle
}

// AddNewUser adds new user using NetUserAdd function of netapi32.dll on local machine
func (u *SessionUtil) AddNewUser(username string, password string) (userExists bool, err error) {
	var (
		errParam uint32
		uPointer *uint16
		pPointer *uint16
	)

	if uPointer, err = syscall.UTF16PtrFromString(username); err != nil {
		return false, fmt.Errorf("Unable to encode username to UTF16")
	}

	if pPointer, err = syscall.UTF16PtrFromString(password); err != nil {
		return false, fmt.Errorf("Unable to encode password to UTF16")
	}

	uInfo := USER_INFO_1{
		Usri1_name:     uPointer,
		Usri1_password: pPointer,
		Usri1_priv:     userPrivUser,
		Usri1_flags:    ufScript,
	}

	ret, _, _ := netUserAdd.Call(
		uintptr(serverNameLocalMachine),
		uintptr(uint32(levelForUserInfo1)),
		uintptr(unsafe.Pointer(&uInfo)),
		uintptr(unsafe.Pointer(&errParam)),
	)

	if ret != nerrSuccess {
		if ret == errCodeForUserAlreadyExists {
			userExists = true
			err = nil
		} else {
			userExists = false
			err = fmt.Errorf("NetUserAdd call failed. Error Code: %d", ret)
		}
	}
	return
}

// IsInstanceADomainController returns true if Active Directory Domain Controller service is running on local instance, false otherwise
func (u *SessionUtil) IsInstanceADomainController(log log.T) (isDCServiceRunning bool) {

	var err error
	if isDCServiceRunning, err = isServiceRunning(addsServiceName); err != nil {
		log.Debugf("Instance is not running active directory domain controller service. %v", err)
		// In case of error always assume instance is not a domain controller machine and return false.
		return false
	}

	return isDCServiceRunning
}

// isServiceRunning returns if given service is running
func isServiceRunning(serviceName string) (isServiceRunning bool, err error) {
	var (
		winManager *mgr.Mgr
		service    *mgr.Service
		status     svc.Status
	)

	// connect to windows service control manager
	if winManager, err = mgr.Connect(); err != nil {
		return false, fmt.Errorf("Something went wrong while trying to connect to Service Manager - %v", err)
	}
	if winManager != nil {
		defer winManager.Disconnect()
	}

	// get handle of service
	if service, err = winManager.OpenService(serviceName); err != nil {
		return false, fmt.Errorf("Opening service failed with error %v", err)
	}
	if service != nil {
		defer service.Close()
	}

	// query to check status of service
	if status, err = service.Query(); err != nil {
		return false, fmt.Errorf("Error when trying to find out if service is running, %v", err)
	}

	if status.State == serviceRunning {
		return true, nil
	}
	return false, nil
}

// AddUserToLocalAdministratorsGroup adds user to local built in administrators group using NetLocalGroupAddMembers function of netapi32.dll
func (u *SessionUtil) AddUserToLocalAdministratorsGroup(username string) (adminGroupName string, err error) {
	var (
		uPointer *uint16
		gPointer *uint16
	)

	if adminGroupName, err = u.getBuiltInAdministratorsGroupName(); err != nil {
		return
	}

	if uPointer, err = syscall.UTF16PtrFromString(username); err != nil {
		return "", fmt.Errorf("Unable to encode username to UTF16")
	}

	if gPointer, err = syscall.UTF16PtrFromString(adminGroupName); err != nil {
		return "", fmt.Errorf("Unable to encode adminGroupName to UTF16")
	}

	localGroupMembers := make([]LOCALGROUP_MEMBERS_INFO_3, 1)
	localGroupMembers[0] = LOCALGROUP_MEMBERS_INFO_3{
		Lgrmi3_domainandname: uPointer,
	}

	ret, _, _ := netLocalGroupAddMembers.Call(
		uintptr(serverNameLocalMachine),
		uintptr(unsafe.Pointer(gPointer)),
		uintptr(uint32(levelForLocalGroupMembersInfo3)),
		uintptr(unsafe.Pointer(&localGroupMembers[0])),
		uintptr(uint32(len(localGroupMembers))),
	)

	// return error if API call failed and user is NOT a group member
	if ret != nerrSuccess && ret != errCodeForUserAlreadyGroupMember {
		err = fmt.Errorf("NetLocalGroupAddMembers call failed. Error Code: %d", ret)
	}

	return
}

// getBuiltInAdministratorsGroupName fetches builtin local administrators group name
func (u *SessionUtil) getBuiltInAdministratorsGroupName() (adminGroupName string, err error) {
	var sid *windows.SID
	if err = windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0,
		0,
		0,
		0,
		0,
		0,
		&sid); err != nil {
		return
	}

	// Passing system name as empty string and LookupAccountSidW will translate it to local system
	if adminGroupName, _, _, err = sid.LookupAccount(""); err != nil {
		return
	}

	return
}

// ChangePassword changes password for given user using NetUserSetInfo function of netapi32.dll on local machine
func (u *SessionUtil) ChangePassword(username string, password string) (userExists bool, err error) {
	var (
		errParam uint32
		uPointer *uint16
		pPointer *uint16
	)

	if uPointer, err = syscall.UTF16PtrFromString(username); err != nil {
		return userExists, fmt.Errorf("Unable to encode username to UTF16")
	}

	if pPointer, err = syscall.UTF16PtrFromString(password); err != nil {
		return userExists, fmt.Errorf("Unable to encode password to UTF16")
	}

	ret, _, _ := netUserSetInfo.Call(
		uintptr(serverNameLocalMachine),
		uintptr(unsafe.Pointer(uPointer)),
		uintptr(uint32(levelForUserInfo1003)),
		uintptr(unsafe.Pointer(&USER_INFO_1003{Usri1003_password: pPointer})),
		uintptr(unsafe.Pointer(&errParam)),
	)

	if ret == nerrSuccess {
		return true, nil
	} else if ret == errCodeForUserNotFound {
		return false, nil
	}
	return userExists, fmt.Errorf("NetUserSetInfo call failed. %d", ret)
}

// ResetPasswordIfDefaultUserExists resets default RunAs user password if user exists (only for agent starts)
func (u *SessionUtil) ResetPasswordIfDefaultUserExists(context context.T) (err error) {
	var userExists bool
	if userExists, err = u.doesUserExist(appconfig.DefaultRunAsUserName); err != nil {
		return fmt.Errorf("Error occured while checking if %s user exists, %v", appconfig.DefaultRunAsUserName, err)
	}

	if userExists {
		log := context.Log()
		log.Infof("%s already exists. Resetting password.", appconfig.DefaultRunAsUserName)
		newPassword, err := u.GeneratePasswordForDefaultUser()
		if err != nil {
			return err
		}
		if _, err = u.ChangePassword(appconfig.DefaultRunAsUserName, newPassword); err != nil {
			return fmt.Errorf("Error occured while changing password for %s, %v", appconfig.DefaultRunAsUserName, err)
		}
	}

	return nil
}

// doesUserExist checks if given user already exists using NetUserGetInfo function of netapi32.dll on local machine
func (u *SessionUtil) doesUserExist(username string) (bool, error) {
	var (
		uPointer         *uint16
		userInfo1Pointer uintptr
		err              error
		userExists       bool
	)

	if uPointer, err = syscall.UTF16PtrFromString(username); err != nil {
		return false, fmt.Errorf("Unable to encode username to UTF16")
	}

	ret, _, _ := netUserGetInfo.Call(
		uintptr(serverNameLocalMachine),
		uintptr(unsafe.Pointer(uPointer)),
		uintptr(uint32(levelForUserInfo1)),
		uintptr(unsafe.Pointer(&userInfo1Pointer)),
	)

	if userInfo1Pointer != nilPointerValue {
		defer netApiBufferFree.Call(uintptr(unsafe.Pointer(userInfo1Pointer)))
	}

	if ret == nerrSuccess {
		userExists = true
	} else if uint(ret) == errCodeForUserNotFound {
		userExists = false
	} else {
		userExists = false
		err = fmt.Errorf("NetUserGetInfo call failed. %d", ret)
	}

	return userExists, err
}

// createLocalAdminUser creates a local OS user on the instance with admin permissions.
func (u *SessionUtil) CreateLocalAdminUser(log log.T) (newPassword string, err error) {
	if u.IsInstanceADomainController(log) {
		return "", fmt.Errorf("Instance is running active directory domain controller service. Disable the service to continue to use session manager.")
	}

	if newPassword, err = u.GeneratePasswordForDefaultUser(); err != nil {
		return
	}

	var userExists bool
	if userExists, err = u.AddNewUser(appconfig.DefaultRunAsUserName, newPassword); err != nil {
		return "", fmt.Errorf("Failed to create %s: %v", appconfig.DefaultRunAsUserName, err)
	}

	if userExists {
		log.Infof("%s already exists.", appconfig.DefaultRunAsUserName)
		return
	}
	log.Infof("Successfully created %s", appconfig.DefaultRunAsUserName)

	var adminGroupName string
	if adminGroupName, err = u.AddUserToLocalAdministratorsGroup(appconfig.DefaultRunAsUserName); err != nil {
		return newPassword, fmt.Errorf("Failed to add %s to local admin group: %v", appconfig.DefaultRunAsUserName, err)
	}
	log.Infof("Added %s to %s group", appconfig.DefaultRunAsUserName, adminGroupName)

	return
}

//Impersonate attempts to impersonate the user.
func (u *SessionUtil) Impersonate(log log.T, user string, pass string) error {
	token, err := logonUser(user, pass)
	if err != nil {
		return err
	}
	defer mustCloseHandle(log, token)
	if rc, _, ec := syscall.Syscall(impersonateProc.Addr(), 1, uintptr(token), 0, 0); rc == 0 {
		return error(ec)
	}
	return nil
}

// LoadUserProfile loads user profile and return user handle to user's registry hive
func (u *SessionUtil) LoadUserProfile(user string, pass string) (syscall.Token, syscall.Handle, error) {
	token, err := logonUser(user, pass)
	if err != nil {
		return 0, 0, err
	}

	var ret uintptr
	var lUser, _ = syscall.UTF16PtrFromString(user)
	var profileInfo = ProfileInfo{UserName: lUser}
	var profileInfoSize = unsafe.Sizeof(profileInfo)
	profileInfo.Size = uint32(profileInfoSize)
	if ret, _, err = loadUserProfileW.Call(uintptr(token), uintptr(unsafe.Pointer(&profileInfo))); ret == 0 {
		return 0, 0, err
	}

	return token, profileInfo.Profile, nil
}

//logonUser attempts to log a user on to the local computer to generate a token.
func logonUser(user, pass string) (token syscall.Token, err error) {
	// ".\0" meaning "this computer:
	domain := [2]uint16{uint16('.'), 0}

	var pu, pp []uint16
	if pu, err = syscall.UTF16FromString(user); err != nil {
		return
	}
	if pp, err = syscall.UTF16FromString(pass); err != nil {
		return
	}

	if rc, _, ec := syscall.Syscall6(logonProc.Addr(), 6,
		uintptr(unsafe.Pointer(&pu[0])),
		uintptr(unsafe.Pointer(&domain[0])),
		uintptr(unsafe.Pointer(&pp[0])),
		logon32LogonNetwork,
		logon32ProviderDefault,
		uintptr(unsafe.Pointer(&token))); rc == 0 {
		err = error(ec)
	}
	return
}

// UnloadUserProfile attempts to unload user profile
func (u *SessionUtil) UnloadUserProfile(log log.T, token syscall.Token, profile syscall.Handle) {
	var ret uintptr
	var err error
	if ret, _, err = unloadUserProfile.Call(uintptr(token), uintptr(profile)); ret == 0 {
		log.Debugf("unloading of user profile failed: ret: %d, %v", ret, err)
	}
	defer mustCloseHandle(log, token)
}

// EnablePowerShellTranscription enabled PowerShell's transcript logging by modifying user's registry hive
func (u *SessionUtil) EnablePowerShellTranscription(transcriptFilePath string, keyHandle syscall.Handle) (err error) {
	winkey, err := registry.OpenKey(registry.Key(keyHandle), `Software\Policies\Microsoft\Windows`, registry.CREATE_SUB_KEY)
	if err != nil {
		return fmt.Errorf("error opening Windows key: %v", err)
	}
	defer winkey.Close()

	powerShellKey, alreadyExists, err := registry.CreateKey(winkey, `PowerShell`, registry.SET_VALUE)
	if err != nil && !alreadyExists {
		return fmt.Errorf("error creating Powershell key: %v", err)
	}
	defer powerShellKey.Close()

	transcriptionKey, alreadyExists, err := registry.CreateKey(powerShellKey, `Transcription`, registry.SET_VALUE)
	if err != nil && !alreadyExists {
		return fmt.Errorf("error creating Transcription key: %v", err)

	}
	defer transcriptionKey.Close()

	err = transcriptionKey.SetDWordValue("EnableTranscripting", 1)
	if err != nil {
		return fmt.Errorf("error setting value for EnableTranscripting key: %v", err)
	}

	err = transcriptionKey.SetStringValue("OutputDirectory", transcriptFilePath)
	if err != nil {
		return fmt.Errorf("error setting value for OutputDirectory key: %v", err)
	}

	return
}

//revertToSelf reverts the impersonation process.
func (u *SessionUtil) RevertToSelf() error {
	if rc, _, ec := syscall.Syscall(revertSelfProc.Addr(), 0, 0, 0, 0); rc == 0 {
		return error(ec)
	}
	return nil
}

//mustCloseHandle ensures to close the user token handle.
func mustCloseHandle(log log.T, token syscall.Token) {
	if err := token.Close(); err != nil {
		log.Error(err)
	}
}

// GetInstalledVersionOfPowerShell fetches installed version of PowerShell
func (u *SessionUtil) GetInstalledVersionOfPowerShell() (powerShellVersion string, err error) {
	powerShellEngineKey, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\PowerShell\3\PowerShellEngine`, registry.READ)
	if err != nil {
		err = fmt.Errorf("error fetching PowerShellEngine registry key: %v", err)
		return
	}
	defer powerShellEngineKey.Close()

	powerShellVersion, _, err = powerShellEngineKey.GetStringValue("PowerShellVersion")
	if err != nil {
		err = fmt.Errorf("error fetching PowerShellVersion registry value: %v", err)
		return
	}
	return
}

// IsSystemLevelPowerShellTranscriptionConfigured checks if PowerShell Transcript logging has been configured at system level
func (u *SessionUtil) IsSystemLevelPowerShellTranscriptionConfigured(log log.T) bool {
	powerShellTranscription, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Policies\Microsoft\Windows\PowerShell\Transcription`, registry.READ)
	if err != nil {
		log.Debugf("error fetching PowerShell Transcription registry key: %v", err)
		return false
	}
	defer powerShellTranscription.Close()

	// return true as PowerShell Transcription is configured
	return true
}

func (u *SessionUtil) EnableLocalUser(log log.T) (err error) {
	if err = u.userDelFlags(log, appconfig.DefaultRunAsUserName, USER_UF_ACCOUNTDISABLE); err != nil {
		log.Errorf("error occurred enabling %s: %v", appconfig.DefaultRunAsUserName, err)
		return err
	}

	log.Infof("Successfully enabled %s", appconfig.DefaultRunAsUserName)
	return nil
}

func (u *SessionUtil) DisableLocalUser(log log.T) (err error) {
	if err = u.userAddFlags(log, appconfig.DefaultRunAsUserName, USER_UF_ACCOUNTDISABLE); err != nil {
		log.Errorf("error occurred disabling %s: %v", appconfig.DefaultRunAsUserName, err)
		return err
	}

	log.Infof("Successfully disabled %s", appconfig.DefaultRunAsUserName)
	return nil
}

func (u *SessionUtil) userAddFlags(log log.T, username string, flags uint32) error {
	eFlags, err := u.userGetFlags(log, username)
	if err != nil {
		return fmt.Errorf("Error while getting existing flags, %s.", err.Error())
	}
	eFlags |= flags // add supplied bits to mask.
	return u.userSetFlags(log, username, eFlags)
}

func (u *SessionUtil) userSetFlags(log log.T, username string, flags uint32) error {
	var errParam uint32
	uPointer, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return fmt.Errorf("Unable to encode username to UTF16")
	}
	ret, _, _ := netUserSetInfo.Call(
		uintptr(serverNameLocalMachine),
		uintptr(unsafe.Pointer(uPointer)),
		uintptr(uint32(levelForUserInfo1008)),
		uintptr(unsafe.Pointer(&USER_INFO_1008{Usri1008_flags: flags})),
		uintptr(unsafe.Pointer(&errParam)),
	)
	if ret != nerrSuccess {
		return fmt.Errorf("NetUserSetInfo call failed when trying to set flags. %d", ret)
	}
	return nil
}

func (u *SessionUtil) userDelFlags(log log.T, username string, flags uint32) error {
	eFlags, err := u.userGetFlags(log, username)
	if err != nil {
		return fmt.Errorf("Error while getting existing flags, %s.", err.Error())
	}
	eFlags &^= flags // clear bits we want to remove.
	return u.userSetFlags(log, username, eFlags)
}

func (u *SessionUtil) userGetFlags(log log.T, username string) (uint32, error) {
	var dataPointer uintptr
	uPointer, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return 0, fmt.Errorf("unable to encode username to UTF16")
	}
	_, _, _ = netUserGetInfo.Call(
		uintptr(serverNameLocalMachine),
		uintptr(unsafe.Pointer(uPointer)),
		uintptr(uint32(levelForUserInfo1)),
		uintptr(unsafe.Pointer(&dataPointer)),
	)
	defer netApiBufferFree.Call(dataPointer)

	if dataPointer == uintptr(0) {
		return 0, fmt.Errorf("unable to get data structure for user flags")
	}

	var data = (*USER_INFO_1)(unsafe.Pointer(dataPointer))
	log.Debugf("existing user flags: %d\r\n", data.Usri1_flags)
	return data.Usri1_flags, nil
}

// NewListener starts a new socket listener on the address.
// unix sockets are not supported in older windows versions, start tcp loopback server in such cases
func NewListener(log log.T, address string) (listener net.Listener, err error) {
	if listener, err = net.Listen("unix", address); err != nil {
		log.Debugf("Failed to open unix socket listener, %v. Starting TCP listener.", err)
		return net.Listen("tcp", "localhost:0")
	}
	return
}

// DeleteIpcTempFile resets file properties of ipcTempFile and tries deletion
func (u *SessionUtil) DeleteIpcTempFile(sessionOrchestrationPath string) (bool, error) {
	// no deletion needed for windows since ipcTempFile properties are default and no issues during deletion
	return false, nil
}
