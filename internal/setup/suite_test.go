package setup_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/rebornplusplus/chisel/internal/setup"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	setup.SetDebug(true)
	setup.SetLogger(c)
}

func (s *S) TearDownTest(c *C) {
	setup.SetDebug(false)
	setup.SetLogger(nil)
}
