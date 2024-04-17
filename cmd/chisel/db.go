package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/jsonwall"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/slicer"
	"github.com/klauspost/compress/zstd"
)

const dbFile = "chisel.db"
const dbSchema = "1.0"
const dbMode = 0644

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
	dbWriters := make(map[string]*jsonwall.DBWriter)
	// Paths to generate DB at.
	dbPaths := make(map[string]string)
	// Path entry for the DB itself.
	dbPathEntries := []*Path{}

	for path, slices := range opts.ManifestInfo {
		dbWriters[path] = jsonwall.NewDBWriter(&jsonwall.DBWriterOptions{
			Schema: dbSchema,
		})
		dbPath := filepath.Join(opts.RootDir, strings.TrimRight(path, "*"), dbFile)
		dbPaths[path] = filepath.Clean(dbPath)

		slicesNames := []string{}
		for _, s := range slices {
			slicesNames = append(slicesNames, s.String())
		}
		relPath := filepath.Clean("/" + strings.TrimPrefix(dbPaths[path], opts.RootDir))
		dbPathEntries = append(dbPathEntries, &Path{
			Kind:   "path",
			Path:   relPath,
			Mode:   fmt.Sprintf("0%o", dbMode&fs.ModePerm),
			Slices: slicesNames,
		})
	}

	generatedPaths := []string{}
	for path, dbw := range dbWriters {
		// Add packages to the DB.
		for _, info := range opts.PackageInfo {
			err := dbw.Add(&Package{
				Kind:    "package",
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
			err := dbw.Add(&Slice{
				Kind: "slice",
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
				err := dbw.Add(&Content{
					Kind:  "content",
					Slice: name,
					Path:  entry.Path,
				})
				if err != nil {
					return nil, err
				}
				sliceNames = append(sliceNames, name)
			}
			err := dbw.Add(&Path{
				Kind:   "path",
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
		for _, pathEntry := range dbPathEntries {
			err := dbw.Add(pathEntry)
			if err != nil {
				return nil, err
			}
			for _, s := range pathEntry.Slices {
				err := dbw.Add(&Content{
					Kind:  "content",
					Slice: s,
					Path:  pathEntry.Path,
				})
				if err != nil {
					return nil, err
				}
			}
		}

		dbPath := dbPaths[path]
		logf("Generating Chisel DB at %s...", dbPath)
		err := WriteDB(dbw, dbPath, dbMode)
		if err != nil {
			return nil, err
		}
		relPath := filepath.Clean("/" + strings.TrimPrefix(dbPath, opts.RootDir))
		generatedPaths = append(generatedPaths, relPath)
	}

	return generatedPaths, nil
}

type Package struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Digest  string `json:"sha256"`
	Arch    string `json:"arch"`
}

type Slice struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type Path struct {
	Kind      string   `json:"kind"`
	Path      string   `json:"path"`
	Mode      string   `json:"mode"`
	Slices    []string `json:"slices"`
	Hash      string   `json:"sha256,omitempty"`
	FinalHash string   `json:"final_sha256,omitempty"`
	Size      uint64   `json:"size,omitempty"`
	Link      string   `json:"link,omitempty"`
}

type Content struct {
	Kind  string `json:"kind"`
	Slice string `json:"slice"`
	Path  string `json:"path"`
}

// WriteDB writes all added entries to the Chisel DB and generates the actual
// file. It returns the path of the generated Chisel DB file. The file
// chisel.db is a zstd compressed file.
func WriteDB(writer *jsonwall.DBWriter, path string, mode fs.FileMode) (err error) {
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	debugf("writing DB at %s...", path)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer file.Close()

	w, err := zstd.NewWriter(file)
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = writer.WriteTo(w)
	if err != nil {
		return err
	}
	return nil
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
