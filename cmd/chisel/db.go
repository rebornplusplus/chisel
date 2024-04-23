package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/jsonwall"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/slicer"
)

const dbFile = "chisel.db"
const dbSchema = "1.0"
const dbMode = 0644

type dbPackage struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Digest  string `json:"sha256"`
	Arch    string `json:"arch"`
}

type dbSlice struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type dbPath struct {
	Kind      string   `json:"kind"`
	Path      string   `json:"path"`
	Mode      string   `json:"mode"`
	Slices    []string `json:"slices"`
	Hash      string   `json:"sha256,omitempty"`
	FinalHash string   `json:"final_sha256,omitempty"`
	Size      uint64   `json:"size,omitempty"`
	Link      string   `json:"link,omitempty"`
}

type dbContent struct {
	Kind  string `json:"kind"`
	Slice string `json:"slice"`
	Path  string `json:"path"`
}

type generateManifestOptions struct {
	// Map of slices indexed by paths which generate manifest.
	ManifestSlices map[string][]*setup.Slice
	// List of package information to write to manifest.
	PackageInfo []*archive.PackageInfo
	// List of slices to write to manifest.
	Slices []*setup.Slice
	// Path entries to write to manifest.
	Report *slicer.Report
}

// generateManifest generates the Chisel manifest(s) at the specified paths. It
// returns the paths inside the rootfs where the manifest(s) are generated.
func generateManifest(opts *generateManifestOptions) (*jsonwall.DBWriter, error) {
	dbw := jsonwall.NewDBWriter(&jsonwall.DBWriterOptions{
		Schema: dbSchema,
	})

	// Add packages to the manifest.
	for _, info := range opts.PackageInfo {
		err := dbw.Add(&dbPackage{
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
	// Add slices to the manifest.
	for _, s := range opts.Slices {
		err := dbw.Add(&dbSlice{
			Kind: "slice",
			Name: s.String(),
		})
		if err != nil {
			return nil, err
		}
	}
	// Add paths and contents to the manifest.
	for _, entry := range opts.Report.Entries {
		sliceNames := []string{}
		for s := range entry.Slices {
			err := dbw.Add(&dbContent{
				Kind:  "content",
				Slice: s.String(),
				Path:  entry.Path,
			})
			if err != nil {
				return nil, err
			}
			sliceNames = append(sliceNames, s.String())
		}
		sort.Strings(sliceNames)
		err := dbw.Add(&dbPath{
			Kind:   "path",
			Path:   entry.Path,
			Mode:   fmt.Sprintf("0%o", entry.Mode&fs.ModePerm),
			Slices: sliceNames,
			Hash:   entry.Hash,
			Size:   uint64(entry.Size),
			Link:   entry.Link,
		})
		if err != nil {
			return nil, err
		}
	}
	// Add the manifest path and content entries.
	for path, slices := range opts.ManifestSlices {
		fPath := getManifestPath(path)
		sliceNames := []string{}
		for _, s := range slices {
			err := dbw.Add(&dbContent{
				Kind:  "content",
				Slice: s.String(),
				Path:  fPath,
			})
			if err != nil {
				return nil, err
			}
			sliceNames = append(sliceNames, s.String())
		}
		sort.Strings(sliceNames)
		err := dbw.Add(&dbPath{
			Kind:   "path",
			Path:   fPath,
			Mode:   fmt.Sprintf("0%o", dbMode&fs.ModePerm),
			Slices: sliceNames,
		})
		if err != nil {
			return nil, err
		}
	}

	return dbw, nil
}

// getManifestPath parses the "generate" path and returns the absolute path of
// the location to be generated.
func getManifestPath(generatePath string) string {
	dir := filepath.Clean(strings.TrimSuffix(generatePath, "**"))
	return filepath.Join(dir, dbFile)
}
