package osfs

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/test"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type OSSuite struct {
	test.FilesystemSuite
	path string
}

var _ = Suite(&OSSuite{})

func (s *OSSuite) SetUpTest(c *C) {
	s.path, _ = ioutil.TempDir(os.TempDir(), "go-billy-osfs-test")
	if runtime.GOOS == "plan9" {
		// On Plan 9, permission mode of newly created files
		// or directories are based on the permission mode of
		// the containing directory (see http://man.cat-v.org/plan_9/5/open).
		// Since TestOpenFileWithModes and TestStat creates files directly
		// in the temporary directory, we need to make it more permissive.
		c.Assert(os.Chmod(s.path, 0777), IsNil)
	}
	s.FilesystemSuite = test.NewFilesystemSuite(New(s.path))
}

func (s *OSSuite) TearDownTest(c *C) {
	err := os.RemoveAll(s.path)
	c.Assert(err, IsNil)
}

func (s *OSSuite) TestOpenDoesNotCreateDir(c *C) {
	_, err := s.FS.Open("dir/non-existent")
	c.Assert(err, NotNil)

	_, err = os.Stat(filepath.Join(s.path, "dir"))
	c.Assert(os.IsNotExist(err), Equals, true)
}

func (s *OSSuite) TestCapabilities(c *C) {
	_, ok := s.FS.(billy.Capable)
	c.Assert(ok, Equals, true)

	caps := billy.Capabilities(s.FS)
	c.Assert(caps, Equals, billy.AllCapabilities)
}
