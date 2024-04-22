package main

import (
	"fmt"
	"io"
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

type generateDBOptions struct {
	// The root dir of the fs.
	RootDir string
	// Map of slices indexed by paths which generate manifest.
	ManifestSlices map[string][]*setup.Slice
	// List of package information to write to manifest.
	PackageInfo []*archive.PackageInfo
	// List of slices to write to manifest.
	Slices []*setup.Slice
	// Path entries to write to manifest.
	Report *slicer.Report
}

// generateDB generates the Chisel manifest(s) at the specified paths. It
// returns the paths inside the rootfs where the manifest(s) are generated.
func generateDB(opts *generateDBOptions) ([]string, error) {
	dbw := jsonwall.NewDBWriter(&jsonwall.DBWriterOptions{
		Schema: dbSchema,
	})
	genPaths := []string{}

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
			// Add contents to the DB.
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
		genPaths = append(genPaths, fPath)
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

	filePaths := []string{}
	for _, path := range genPaths {
		filePaths = append(filePaths, filepath.Join(opts.RootDir, path))
	}
	err := writeDB(dbw, filePaths)
	if err != nil {
		return nil, err
	}
	return genPaths, nil
}

// writeDB writes all added entries and generates the manifest file.
func writeDB(writer *jsonwall.DBWriter, paths []string) (err error) {
	files := []io.Writer{}
	for _, path := range paths {
		if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		logf("Generating manifest at %s...", path)
		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, dbMode)
		if err != nil {
			return err
		}
		files = append(files, file)
		defer file.Close()
	}

	// Using a MultiWriter allows to compress the data only once and write the
	// compressed data to each path.
	multiWriter := io.MultiWriter(files...)
	w, err := zstd.NewWriter(multiWriter)
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = writer.WriteTo(w)
	return err
}

func getManifestPath(generatePath string) string {
	return filepath.Join(strings.TrimRight(generatePath, "*"), dbFile)
}

// locateManifestSlices finds the paths marked with "generate:manifest" and
// returns a map from said path to all the slices that declare it.
func locateManifestSlices(slices []*setup.Slice) map[string][]*setup.Slice {
	manifestSlices := make(map[string][]*setup.Slice)
	for _, s := range slices {
		for path, info := range s.Contents {
			if info.Generate == setup.GenerateManifest {
				if manifestSlices[path] == nil {
					manifestSlices[path] = []*setup.Slice{}
				}
				manifestSlices[path] = append(manifestSlices[path], s)
			}
		}
	}
	return manifestSlices
}
