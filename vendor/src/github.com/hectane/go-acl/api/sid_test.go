package api

import (
	"golang.org/x/sys/windows"

	"testing"
	"unsafe"
)

func TestSIDLookup(t *testing.T) {
	var (
		sid    = make([]byte, SECURITY_MAX_SID_SIZE)
		sidLen = uint32(unsafe.Sizeof(sid))
	)
	if err := CreateWellKnownSid(
		WinNullSid,
		nil,
		(*windows.SID)(unsafe.Pointer(&sid[0])),
		&sidLen,
	); err != nil {
		t.Fatal(err)
	}
}
