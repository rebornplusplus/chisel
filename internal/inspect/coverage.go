package inspect

import (
	"fmt"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/strdist"
)

type CoverageOptions struct {
	Release  *setup.Release
	Slices   []string
	Archives map[string]archive.Archive

	IgnoreDeps bool
}

type CoverageProperties struct {
	// This indicates the package related to the covered path. This is not a
	// slice because Chisel does not allow conflicting paths across packages.
	Package string
	// This slice indicates the list of slices that covered a path. For paths
	// that were not included in any slice (omitted paths), this slice is empty.
	Slices []string
	// This slices contains the matching slice path entries e.g. globs, copy
	// paths. Since multiple globs within a package can match a path, this is a
	// slice.
	SlicePaths []string
}

type Coverage struct {
	// Contains info about package paths that were matched by query slices.
	Matched map[string]*CoverageProperties
	// Contains info about package paths that were not covered by any query
	// slice.
	Omitted map[string]*CoverageProperties
	// Contains info about paths that were added by query slices that do not
	// exist in the corresponding packages.
	Added map[string]*CoverageProperties
}

// ReportCoverage reports the coverage of package paths by query slices. It
// includes information of which package paths are matched by slice entries,
// which are omitted and which entries are added (but unmatched) by the slices.
func ReportCoverage(opts *CoverageOptions) (*Coverage, error) {
	pkgs, slices, err := determinePkgSlices(opts.Release, opts.Slices, opts.IgnoreDeps)
	if err != nil {
		return nil, err
	}
	archives, err := groupArchives(opts.Archives, pkgs)
	if err != nil {
		return nil, err
	}
	pkgPaths := make(map[string][]string)
	for _, pkg := range pkgs {
		paths, err := listPkgPaths(archives[pkg.Name], pkg.Name)
		if err != nil {
			return nil, err
		}
		pkgPaths[pkg.Name] = paths
	}
	return findCoverage(slices, pkgPaths)
}

func findCoverage(slices []*setup.Slice, pkgPaths map[string][]string) (*Coverage, error) {
	coverage := &Coverage{
		Matched: make(map[string]*CoverageProperties),
		Omitted: make(map[string]*CoverageProperties),
		Added:   make(map[string]*CoverageProperties),
	}

	pkgSlices := make(map[string][]*setup.Slice)
	for _, slice := range slices {
		pkgSlices[slice.Package] = append(pkgSlices[slice.Package], slice)
	}

	addMatched := func(path string, attr *CoverageProperties) {
		entry, ok := coverage.Matched[path]
		if !ok {
			coverage.Matched[path] = attr
			return
		}
		if attr.Slices != nil {
			entry.Slices = append(entry.Slices, attr.Slices...)
		}
		if attr.SlicePaths != nil {
			entry.SlicePaths = append(entry.SlicePaths, attr.SlicePaths...)
		}
	}

	for pkg, slices := range pkgSlices {
		paths, ok := pkgPaths[pkg]
		if !ok {
			return nil, fmt.Errorf("internal error: package %s paths not found", pkg)
		}
		entriesMatched := make(map[string]bool)
		for _, sourcePath := range paths {
			var matched bool
			for _, slice := range slices {
				for pathEntry, pathInfo := range slice.Contents {
					if (pathInfo.Kind == setup.CopyPath && pathEntry == sourcePath) ||
						(pathInfo.Kind == setup.GlobPath && strdist.GlobPath(pathEntry, sourcePath)) {
						addMatched(sourcePath, &CoverageProperties{
							Package:    pkg,
							Slices:     []string{slice.Name},
							SlicePaths: []string{pathEntry},
						})
						matched = true
						entriesMatched[pathEntry] = true
					}
				}
			}
			if !matched {
				coverage.Omitted[sourcePath] = &CoverageProperties{
					Package: pkg,
				}
			}
		}
		for _, slice := range slices {
			for pathEntry := range slice.Contents {
				if _, ok := entriesMatched[pathEntry]; !ok {
					coverage.Added[pathEntry] = &CoverageProperties{
						Package:    pkg,
						Slices:     []string{slice.Name},
						SlicePaths: []string{pathEntry},
					}
				}
			}
		}
	}
	return coverage, nil
}

func determinePkgSlices(release *setup.Release, slices []string, ignoreDeps bool) ([]*setup.Package, []*setup.Slice, error) {
	keys := make([]setup.SliceKey, 0, len(slices))
	for _, slice := range slices {
		key, err := setup.ParseSliceKey(slice)
		if err != nil {
			// If the query is only a package name, include all slices.
			pkg, ok := release.Packages[slice]
			if !ok {
				return nil, nil, err
			}
			for _, s := range pkg.Slices {
				keys = append(keys, setup.SliceKey{Package: pkg.Name, Slice: s.Name})
			}
			continue
		}
		keys = append(keys, key)
	}

	if !ignoreDeps {
		var err error
		keys, err = setup.Order(release.Packages, keys)
		if err != nil {
			return nil, nil, err
		}
	}

	seen := make(map[string]bool)
	var allPackages []*setup.Package
	allSlices := make([]*setup.Slice, 0, len(keys))
	for _, key := range keys {
		pkg := release.Packages[key.Package]
		allSlices = append(allSlices, pkg.Slices[key.Slice])
		if !seen[pkg.Name] {
			seen[pkg.Name] = true
			allPackages = append(allPackages, pkg)
		}
	}
	return allPackages, allSlices, nil
}

// Selects and groups archives by package name.
func groupArchives(archives map[string]archive.Archive, pkgs []*setup.Package) (map[string]archive.Archive, error) {
	pkgArchives := make(map[string]archive.Archive)
	for _, pkg := range pkgs {
		archive := archives[pkg.Archive]
		if archive == nil {
			return nil, fmt.Errorf("archive %q not defined", pkg.Archive)
		}
		if !archive.Exists(pkg.Name) {
			return nil, fmt.Errorf("slice package %q missing from archive", pkg.Name)
		}
		pkgArchives[pkg.Name] = archive
	}
	return pkgArchives, nil
}

func listPkgPaths(archive archive.Archive, pkg string) ([]string, error) {
	reader, err := archive.Fetch(pkg)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	paths, err := deb.List(reader)
	if err != nil {
		return nil, err
	}
	return paths, nil
}
