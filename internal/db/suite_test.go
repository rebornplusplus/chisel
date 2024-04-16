package db_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/db"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	db.SetDebug(true)
	db.SetLogger(c)
}

func (s *S) TearDownTest(c *C) {
	db.SetDebug(false)
	db.SetLogger(nil)
}
