package api

import (
	"golang.org/x/sys/windows"

	"io/ioutil"
	"os"
	"testing"
)

func TestGetNamedSecurityInfo(t *testing.T) {
	f, err := ioutil.TempFile(os.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	var (
		secDesc windows.Handle
	)
	if err = GetNamedSecurityInfo(
		f.Name(),
		SE_FILE_OBJECT,
		0,
		nil,
		nil,
		nil,
		nil,
		&secDesc,
	); err != nil {
		t.Fatal(err)
	}
	defer windows.LocalFree(secDesc)
}

func TestSetNamedSecurityInfo(t *testing.T) {
	f, err := ioutil.TempFile(os.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if err = SetNamedSecurityInfo(
		f.Name(),
		SE_FILE_OBJECT,
		DACL_SECURITY_INFORMATION,
		nil,
		nil,
		0,
		0,
	); err != nil {
		t.Fatal(err)
	}
}
