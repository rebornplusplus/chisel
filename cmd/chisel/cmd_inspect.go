package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jessevdk/go-flags"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/cache"
	"github.com/canonical/chisel/internal/inspect"
)

var shortInspectHelp = "Inspect slices"
var longInspectHelp = `
The inspect command inspects slice(s) and provides useful info.
`

var inspectDescs = map[string]string{
	"release":         "Chisel release name or directory (e.g. ubuntu-22.04)",
	"arch":            "Package architecture",
	"ignore-deps":     "Ignore slice dependency",
	"no-coverage":     "Do not show package coverage",
	"no-matched":      "Do not show matched coverage",
	"no-omitted":      "Do not show omitted coverage",
	"no-added":        "Do not show added coverage",
	"no-omitted-dirs": "Do not show omitted directories",
}

type cmdInspect struct {
	Release string `long:"release" value-name:"<branch|dir>"`
	Arch    string `long:"arch" value-name:"<arch>"`

	// slice deps
	IgnoreDeps bool `long:"ignore-deps"`

	// coverage
	NoCoverage    bool `long:"no-coverage"`
	NoMatched     bool `long:"no-matched"`
	NoOmitted     bool `long:"no-omitted"`
	NoAdded       bool `long:"no-added"`
	NoOmittedDirs bool `long:"no-omitted-dirs"`

	Positional struct {
		SliceRefs []string `positional-arg-name:"pkg/slice" required:"yes"`
	} `positional-args:"yes"`
}

func init() {
	addCommand(
		"inspect",
		shortInspectHelp,
		longInspectHelp,
		func() flags.Commander { return &cmdInspect{} },
		inspectDescs,
		nil,
	)
}

func (cmd *cmdInspect) Execute(args []string) error {
	if len(args) > 0 {
		return ErrExtraArgs
	}

	release, err := obtainRelease(cmd.Release)
	if err != nil {
		return err
	}

	var archives map[string]archive.Archive
	needArchives := !cmd.NoCoverage
	if needArchives {
		archives = make(map[string]archive.Archive)
		for archiveName, archiveInfo := range release.Archives {
			openArchive, err := archive.Open(&archive.Options{
				Label:      archiveName,
				Version:    archiveInfo.Version,
				Arch:       cmd.Arch,
				Suites:     archiveInfo.Suites,
				Components: archiveInfo.Components,
				CacheDir:   cache.DefaultDir("chisel"),
				PubKeys:    archiveInfo.PubKeys,
			})
			if err != nil {
				return err
			}
			archives[archiveName] = openArchive
		}
	}

	if !cmd.NoCoverage {
		err := cmd.showCoverage(&inspect.CoverageOptions{
			Release:    release,
			Slices:     cmd.Positional.SliceRefs,
			Archives:   archives,
			IgnoreDeps: cmd.IgnoreDeps,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (cmd *cmdInspect) showCoverage(opts *inspect.CoverageOptions) error {
	coverage, err := inspect.ReportCoverage(opts)
	if err != nil {
		return fmt.Errorf("cannot show coverage: %w", err)
	}

	sortPaths := func(pathAttr map[string]*inspect.CoverageProperties) []string {
		pkgPaths := make(map[string][]string)
		var pkgs []string
		for path, attr := range pathAttr {
			if len(pkgPaths[attr.Package]) == 0 {
				pkgs = append(pkgs, attr.Package)
			}
			pkgPaths[attr.Package] = append(pkgPaths[attr.Package], path)
		}
		slices.Sort(pkgs)
		var paths []string
		for _, pkg := range pkgs {
			slices.Sort(pkgPaths[pkg])
			paths = append(paths, pkgPaths[pkg]...)
		}
		return paths
	}

	w := tabWriter()
	defer w.Flush()

	if len(coverage.Matched) > 0 {
		fmt.Fprintf(w, "-- MATCHED --\tPackage\tSlices\tEntries\n")
		paths := sortPaths(coverage.Matched)
		for _, path := range paths {
			attr := coverage.Matched[path]
			fmt.Fprintf(w, "%s\t%s\t%v\t%v\n", path, attr.Package, attr.Slices, attr.SlicePaths)
		}
		w.Flush()
	}
	if len(coverage.Added) > 0 {
		fmt.Fprintf(w, "-- ADDED --\tPackage\tSlices\tEntries\n")
		paths := sortPaths(coverage.Added)
		for _, path := range paths {
			attr := coverage.Added[path]
			fmt.Fprintf(w, "%s\t%s\t%v\t%v\n", path, attr.Package, attr.Slices, attr.SlicePaths)
		}
		w.Flush()
	}
	if len(coverage.Omitted) > 0 {
		fmt.Fprintf(w, "-- OMITTED --\tPackage\n")
		paths := sortPaths(coverage.Omitted)
		for _, path := range paths {
			if cmd.NoOmittedDirs && strings.HasSuffix(path, "/") {
				continue
			}
			attr := coverage.Omitted[path]
			fmt.Fprintf(w, "%s\t%s\n", path, attr.Package)
		}
		w.Flush()
	}

	return nil
}
