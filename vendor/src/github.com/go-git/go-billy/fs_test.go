package billy_test

import (
	"testing"

	. "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/test"

	. "gopkg.in/check.v1"
)

type FSSuite struct{}

func Test(t *testing.T) { TestingT(t) }

var _ = Suite(&FSSuite{})

func (s *FSSuite) TestCapabilities(c *C) {
	cases := []struct {
		caps     Capability
		expected bool
	}{
		{LockCapability, false},
		{ReadCapability, true},
		{ReadCapability | WriteCapability, true},
		{ReadCapability | WriteCapability | ReadAndWriteCapability | TruncateCapability, true},
		{ReadCapability | WriteCapability | ReadAndWriteCapability | TruncateCapability | LockCapability, false},
		{TruncateCapability | LockCapability, false},
	}

	// This filesystem supports all capabilities except for LockCapability
	fs := new(test.NoLockCapFs)

	for _, e := range cases {
		c.Assert(CapabilityCheck(fs, e.caps), Equals, e.expected)
	}

	dummy := new(test.BasicMock)
	c.Assert(Capabilities(dummy), Equals, DefaultCapabilities)
}
