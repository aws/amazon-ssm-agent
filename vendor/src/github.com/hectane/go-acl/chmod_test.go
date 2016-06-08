package acl

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestChmod(t *testing.T) {
	f, err := ioutil.TempFile(os.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if err := Chmod(f.Name(), 0); err != nil {
		t.Fatal(err)
	}
	r, err := os.Open(f.Name())
	if err == nil {
		r.Close()
		t.Fatal("owner able to access file", f.Name())
	}
	if err := Chmod(f.Name(), 0400); err != nil {
		t.Fatal(err)
	}
	r, err = os.Open(f.Name())
	if err != nil {
		t.Fatal("owner unable to access file")
	}
	r.Close()
}
