package acl

import (
	"golang.org/x/sys/windows"

	"io/ioutil"
	"os"
	"testing"
)

func TestApply(t *testing.T) {
	f, err := ioutil.TempFile(os.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if err := Apply(
		f.Name(),
		true,
		true,
		DenyName(windows.GENERIC_ALL, "CREATOR OWNER"),
	); err != nil {
		t.Fatal(err)
	}
	r, err := os.Open(f.Name())
	if err == nil {
		r.Close()
		t.Fatal("owner able to access file")
	}
}
