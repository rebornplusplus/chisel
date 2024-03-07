package slicer

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/setup"
)

type ReportEntry struct {
	Path   string
	Mode   fs.FileMode
	Hash   string
	Size   int
	Slices map[*setup.Slice]bool
	Link   string

	Mutated   bool
	FinalHash string
}

// Report holds the information about files and directories created when slicing
// packages.
type Report struct {
	// Root is the filesystem path where the all reported content is based.
	Root string
	// Entries holds all reported content, indexed by their path.
	Entries map[string]ReportEntry
}

// NewReport returns an empty report for content that will be based at the
// provided root path.
func NewReport(root string) *Report {
	return &Report{
		Root:    filepath.Clean(root) + "/",
		Entries: make(map[string]ReportEntry),
	}
}

func (r *Report) Add(slice *setup.Slice, fsEntry *fsutil.Entry) error {
	relPath, err := r.relativePath(fsEntry.Path, fsEntry.Mode.IsDir())
	if err != nil {
		return fmt.Errorf("cannot add path: %w", err)
	}

	if entry, ok := r.Entries[relPath]; ok {
		if fsEntry.Mode != entry.Mode {
			return fmt.Errorf("path %q reported twice with diverging mode: %q != %q", relPath, fsEntry.Mode, entry.Mode)
		} else if fsEntry.Link != entry.Link {
			return fmt.Errorf("path %q reported twice with diverging link: %q != %q", relPath, fsEntry.Link, entry.Link)
		} else if fsEntry.Size != entry.Size {
			return fmt.Errorf("path %q reported twice with diverging size: %d != %d", relPath, fsEntry.Size, entry.Size)
		} else if fsEntry.Hash != entry.Hash {
			return fmt.Errorf("path %q reported twice with diverging hash: %q != %q", relPath, fsEntry.Hash, entry.Hash)
		}
		entry.Slices[slice] = true
		r.Entries[relPath] = entry
	} else {
		r.Entries[relPath] = ReportEntry{
			Path:   relPath,
			Mode:   fsEntry.Mode,
			Hash:   fsEntry.Hash,
			Size:   fsEntry.Size,
			Slices: map[*setup.Slice]bool{slice: true},
			Link:   fsEntry.Link,
		}
	}
	return nil
}

// AddMutated updates the initial entry of a mutated path with the final values
// after mutation. It only updates FinalHash and Size. It assumes that an entry
// already exists with the other values. AddMutated can be called at most once
// for a path.
func (r *Report) AddMutated(fsEntry *fsutil.Entry) error {
	relPath, err := r.relativePath(fsEntry.Path, fsEntry.Mode.IsDir())
	if err != nil {
		return fmt.Errorf("cannot add path: %w", err)
	}

	entry, ok := r.Entries[relPath]
	if !ok {
		return fmt.Errorf("path %q has not been added before", relPath)
	}
	if entry.Mutated {
		return fmt.Errorf("path %q has been mutated once before", relPath)
	}
	entry.Mutated = true
	// Only update FinalHash and Size as mutation scripts only changes those.
	entry.FinalHash = fsEntry.Hash
	entry.Size = fsEntry.Size
	r.Entries[relPath] = entry
	return nil
}

func (r *Report) relativePath(path string, isDir bool) (string, error) {
	if !strings.HasPrefix(path, r.Root) {
		return "", fmt.Errorf("%q outside of root %q", path, r.Root)
	}
	relPath := filepath.Clean("/" + strings.TrimPrefix(path, r.Root))
	if isDir {
		relPath = relPath + "/"
	}
	return relPath, nil
}
