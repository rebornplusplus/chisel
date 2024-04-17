package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/db"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/slicer"
)

type GenerateDBOptions struct {
	// The root dir of the fs.
	RootDir string
	// Map of slices indexed by paths which generate manifest.
	ManifestInfo map[string][]*setup.Slice
	// List of package information to write to Chisel DB.
	PackageInfo []*archive.PackageInfo
	// List of slices to write to Chisel DB.
	Slices []*setup.Slice
	// Path entries to write to Chisel DB.
	Report *slicer.Report
}

// GenerateDB generates the Chisel DB(s) at the specified paths. It returns the
// paths inside the rootfs where the DB(s) are generated.
func GenerateDB(opts *GenerateDBOptions) ([]string, error) {
	dbWriters := make(map[string]*db.DBWriter)
	for path := range opts.ManifestInfo {
		dir := filepath.Join(opts.RootDir, strings.TrimRight(path, "*")) + "/"
		dbWriters[path] = db.NewDBWriter(dir)
	}

	// Path entry for the DB itself.
	dbPaths := []*db.Path{}
	for path, slices := range opts.ManifestInfo {
		slicesNames := []string{}
		for _, s := range slices {
			slicesNames = append(slicesNames, s.String())
		}
		dbw := dbWriters[path]
		relPath := filepath.Clean("/" + strings.TrimPrefix(dbw.Path, opts.RootDir))
		dbPaths = append(dbPaths, &db.Path{
			Path:   relPath,
			Mode:   fmt.Sprintf("0%o", dbw.Mode&fs.ModePerm),
			Slices: slicesNames,
		})
	}

	generatedPaths := []string{}
	for _, dbw := range dbWriters {
		// Add packages to the DB.
		for _, info := range opts.PackageInfo {
			err := dbw.AddPackage(&db.Package{
				Name:    info.Name,
				Version: info.Version,
				Digest:  info.Hash,
				Arch:    info.Arch,
			})
			if err != nil {
				return nil, err
			}
		}
		// Add slices to the DB.
		for _, s := range opts.Slices {
			err := dbw.AddSlice(&db.Slice{
				Name: s.String(),
			})
			if err != nil {
				return nil, err
			}
		}
		// Add paths and contents to the DB.
		for _, entry := range opts.Report.Entries {
			mode := fmt.Sprintf("0%o", entry.Mode&fs.ModePerm)
			sliceNames := []string{}
			for s := range entry.Slices {
				name := s.String()
				// Add contents to the DB.
				err := dbw.AddContent(&db.Content{
					Slice: name,
					Path:  entry.Path,
				})
				if err != nil {
					return nil, err
				}
				sliceNames = append(sliceNames, name)
			}
			err := dbw.AddPath(&db.Path{
				Path:   entry.Path,
				Mode:   mode,
				Slices: sliceNames,
				Hash:   entry.Hash,
				Size:   uint64(entry.Size),
				Link:   entry.Link,
			})
			if err != nil {
				return nil, err
			}
		}
		// Add the DB path and content entries.
		for _, dbPath := range dbPaths {
			err := dbw.AddPath(dbPath)
			if err != nil {
				return nil, err
			}
			for _, s := range dbPath.Slices {
				err := dbw.AddContent(&db.Content{
					Slice: s,
					Path:  dbPath.Path,
				})
				if err != nil {
					return nil, err
				}
			}
		}

		logf("Generating Chisel DB at %s...", dbw.Path)
		dbPath, err := dbw.WriteDB()
		if err != nil {
			return nil, err
		}
		relPath := filepath.Clean("/" + strings.TrimPrefix(dbPath, opts.RootDir))
		generatedPaths = append(generatedPaths, relPath)
	}

	return generatedPaths, nil
}

// locateManifests returns a map of slices which contains paths with
// "generate:manifest". Those paths are used as keys of the returning map.
func locateManifests(slices []*setup.Slice) map[string][]*setup.Slice {
	manifestInfo := make(map[string][]*setup.Slice)
	for _, s := range slices {
		for path, info := range s.Contents {
			if info.Generate == setup.GenerateManifest {
				if manifestInfo[path] == nil {
					manifestInfo[path] = []*setup.Slice{}
				}
				if len(manifestInfo[path]) == 0 || manifestInfo[path][len(manifestInfo[path])-1] != s {
					manifestInfo[path] = append(manifestInfo[path], s)
				}
			}
		}
	}
	return manifestInfo
}

// gatherPackageInfo returns a list of PackageInfo for packages who belong to
// the selected slices.
func gatherPackageInfo(selection *setup.Selection, archives map[string]archive.Archive) ([]*archive.PackageInfo, error) {
	if selection == nil {
		return nil, fmt.Errorf("cannot gather package info: selection is nil")
	}
	pkgInfo := []*archive.PackageInfo{}
	done := make(map[string]bool)
	for _, s := range selection.Slices {
		if done[s.Package] {
			continue
		}
		done[s.Package] = true
		archive, ok := archives[s.Package]
		if !ok {
			return nil, fmt.Errorf("no archive found for package %q", s.Package)
		}
		info, err := archive.Info(s.Package)
		if err != nil {
			return nil, err
		}
		pkgInfo = append(pkgInfo, info)
	}
	return pkgInfo, nil
}
