package testutil

import (
	"bytes"
	"fmt"
	"io"

	"github.com/canonical/chisel/internal/archive"
)

type TestArchive struct {
	Opts     archive.Options
	Packages map[string]TestPackage
}

type TestPackage struct {
	Name    string
	Version string
	Hash    string
	Arch    string
	Data    []byte
}

func (a *TestArchive) Options() *archive.Options {
	return &a.Opts
}

func (a *TestArchive) Fetch(pkgName string) (io.ReadCloser, error) {
	if pkg, ok := a.Packages[pkgName]; ok {
		return io.NopCloser(bytes.NewBuffer(pkg.Data)), nil
	}
	return nil, fmt.Errorf("cannot find package %q in archive", pkgName)
}

func (a *TestArchive) Exists(pkg string) bool {
	_, ok := a.Packages[pkg]
	return ok
}
