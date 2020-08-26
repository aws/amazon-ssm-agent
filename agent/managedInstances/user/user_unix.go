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

// +build darwin dragonfly freebsd !android,linux netbsd openbsd solaris

// package user re-implements os/user functions without the use of cgo for unix
package user

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

const (
	PASSWD_PATH       = "/etc/passwd"
	CURRENT_ERROR_MSG = "failed to get the current user from the system user database"

	PASSWD_USERNAME_INDEX = 0
	PASSWD_UID_INDEX      = 2
	PASSWD_GID_INDEX      = 3
	PASSWD_GEOCS_INDEX    = 4
	PASSWD_HOME_DIR_INDEX = 5
)

func current() (*user.User, error) {

	// get current user's UID
	uid := syscall.Getuid()

	// open passwd path
	f, err := os.Open(PASSWD_PATH)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%v - %v", CURRENT_ERROR_MSG, err.Error()))
	}

	defer f.Close()

	// read file by line
	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()

		// parse user from line string
		user, err := parsePasswdUser(line)
		if err != nil {
			continue
		}

		// return user if UIDs match
		if user.Uid == strconv.Itoa(uid) {
			return user, nil
		}
	}

	return nil, errors.New(CURRENT_ERROR_MSG)
}

func parsePasswdUser(passwdUserStr string) (*user.User, error) {
	// format - username:password:UID:GID:GECOS:home_directory:shell
	parsed_str := strings.Split(passwdUserStr, ":")
	if len(parsed_str) != 7 {
		return nil, errors.New("invalid format to parse User")
	}

	if _, err := strconv.Atoi(parsed_str[PASSWD_UID_INDEX]); err != nil {
		return nil, errors.New("invalid UID to parse User")
	}

	if _, err := strconv.Atoi(parsed_str[PASSWD_GID_INDEX]); err != nil {
		return nil, errors.New("invalid GID to parse User")
	}

	return &user.User{
		Username: parsed_str[PASSWD_USERNAME_INDEX], // username
		Uid:      parsed_str[PASSWD_UID_INDEX],      // UID
		Gid:      parsed_str[PASSWD_GID_INDEX],      // GID
		Name:     parsed_str[PASSWD_GEOCS_INDEX],    // GECOS
		HomeDir:  parsed_str[PASSWD_HOME_DIR_INDEX], // home directory
	}, nil

}
