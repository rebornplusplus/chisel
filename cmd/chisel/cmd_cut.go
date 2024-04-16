package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jessevdk/go-flags"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/cache"
	"github.com/canonical/chisel/internal/db"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/slicer"
)

var shortCutHelp = "Cut a tree with selected slices"
var longCutHelp = `
The cut command uses the provided selection of package slices
to create a new filesystem tree in the root location.
`

var cutDescs = map[string]string{
	"release": "Chisel release directory",
	"root":    "Root for generated content",
	"arch":    "Package architecture",
}

type cmdCut struct {
	Release string `long:"release" value-name:"<dir>"`
	RootDir string `long:"root" value-name:"<dir>" required:"yes"`
	Arch    string `long:"arch" value-name:"<arch>"`

	Positional struct {
		SliceRefs []string `positional-arg-name:"<slice names>" required:"yes"`
	} `positional-args:"yes"`
}

func init() {
	addCommand("cut", shortCutHelp, longCutHelp, func() flags.Commander { return &cmdCut{} }, cutDescs, nil)
}

func (cmd *cmdCut) Execute(args []string) error {
	if len(args) > 0 {
		return ErrExtraArgs
	}

	sliceKeys := make([]setup.SliceKey, len(cmd.Positional.SliceRefs))
	for i, sliceRef := range cmd.Positional.SliceRefs {
		sliceKey, err := setup.ParseSliceKey(sliceRef)
		if err != nil {
			return err
		}
		sliceKeys[i] = sliceKey
	}

	var release *setup.Release
	var err error
	if strings.Contains(cmd.Release, "/") {
		release, err = setup.ReadRelease(cmd.Release)
	} else {
		var label, version string
		if cmd.Release == "" {
			label, version, err = readReleaseInfo()
		} else {
			label, version, err = parseReleaseInfo(cmd.Release)
		}
		if err != nil {
			return err
		}
		release, err = setup.FetchRelease(&setup.FetchOptions{
			Label:   label,
			Version: version,
		})
	}
	if err != nil {
		return err
	}

	selection, err := setup.Select(release, sliceKeys)
	if err != nil {
		return err
	}

	archives, err := cmd.packageArchives(release)
	if err != nil {
		return err
	}

	report, err := slicer.Run(&slicer.RunOptions{
		Selection: selection,
		Archives:  archives,
		TargetDir: cmd.RootDir,
	})
	if err != nil {
		return err
	}

	manifestDir, err := findManifestDir(selection.Slices)
	if err == nil {
		manifestDir = filepath.Join(cmd.RootDir, manifestDir) + "/"
		pkgInfo, err := gatherPackageInfo(selection, archives)
		if err != nil {
			return err
		}
		_, err = GenerateDB(&GenerateDBOptions{
			Dir:         manifestDir,
			PackageInfo: pkgInfo,
			Slices:      selection.Slices,
			Report:      report,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// packageArchives returns a map of archives indexed by package names.
func (cmd *cmdCut) packageArchives(release *setup.Release) (map[string]archive.Archive, error) {
	archives := make(map[string]archive.Archive)
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
			return nil, err
		}
		archives[archiveName] = openArchive
	}
	pkgArchives := make(map[string]archive.Archive)
	for _, pkg := range release.Packages {
		if _, ok := pkgArchives[pkg.Name]; ok {
			continue
		}
		archive, err := slicer.PackageArchive(pkg, archives)
		if err != nil {
			return nil, err
		}
		pkgArchives[pkg.Name] = archive
	}
	return pkgArchives, nil
}

// findManifestDir finds the path with "generate: manifest" in the selected
// slices. It returns an error if no such path is found.
func findManifestDir(slices []*setup.Slice) (string, error) {
	for _, s := range slices {
		for path, info := range s.Contents {
			if info.Generate == setup.GenerateManifest {
				return strings.TrimRight(path, "*"), nil
			}
		}
	}
	return "", fmt.Errorf("no path with \"generate: manifest\" found")
}

type GenerateDBOptions struct {
	// The directory where to generate the Chisel DB at.
	Dir string
	// List of package information to write to Chisel DB.
	PackageInfo []archive.PackageInfo
	// List of slices to write to Chisel DB.
	Slices []*setup.Slice
	// Path entries to write to Chisel DB.
	Report *slicer.Report
}

// GenerateDB generates the Chisel DB with the specified options. It returns the
// path of the DB if successful.
func GenerateDB(opts *GenerateDBOptions) (string, error) {
	logf("Generating Chisel DB at %s...", opts.Dir)

	dbw := db.NewDBWriter(opts.Dir)
	// Add packages to the DB.
	for _, info := range opts.PackageInfo {
		err := dbw.AddPackage(&db.Package{
			Name:    info.Name(),
			Version: info.Version(),
			Digest:  info.Hash(),
			Arch:    info.Arch(),
		})
		if err != nil {
			return "", err
		}
	}
	// Add slices to the DB.
	for _, s := range opts.Slices {
		err := dbw.AddSlice(&db.Slice{
			Name: s.String(),
		})
		if err != nil {
			return "", err
		}
	}
	// Add paths to the DB.
	for _, entry := range opts.Report.Entries {
		mode := fmt.Sprintf("0%o", entry.Mode&fs.ModePerm)
		sliceNames := []string{}
		for s := range entry.Slices {
			sliceNames = append(sliceNames, s.String())
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
			return "", err
		}
	}
	// Add contents to the DB.
	for _, entry := range opts.Report.Entries {
		for s := range entry.Slices {
			err := dbw.AddContent(&db.Content{
				Slice: s.String(),
				Path:  entry.Path,
			})
			if err != nil {
				return "", err
			}
		}
	}
	return dbw.WriteDB()
}

// gatherPackageInfo returns a list of PackageInfo for packages who belong to
// the selected slices.
func gatherPackageInfo(selection *setup.Selection, archives map[string]archive.Archive) ([]archive.PackageInfo, error) {
	if selection == nil {
		return nil, fmt.Errorf("cannot gather package info: selection is nil")
	}
	pkgInfo := []archive.PackageInfo{}
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

// TODO These need testing, and maybe moving into a common file.

var releaseExp = regexp.MustCompile(`^([a-z](?:-?[a-z0-9]){2,})-([0-9]+(?:\.?[0-9])+)$`)

func parseReleaseInfo(release string) (label, version string, err error) {
	match := releaseExp.FindStringSubmatch(release)
	if match == nil {
		return "", "", fmt.Errorf("invalid release reference: %q", release)
	}
	return match[1], match[2], nil
}

func readReleaseInfo() (label, version string, err error) {
	data, err := os.ReadFile("/etc/lsb-release")
	if err == nil {
		const labelPrefix = "DISTRIB_ID="
		const versionPrefix = "DISTRIB_RELEASE="
		for _, line := range strings.Split(string(data), "\n") {
			switch {
			case strings.HasPrefix(line, labelPrefix):
				label = strings.ToLower(line[len(labelPrefix):])
			case strings.HasPrefix(line, versionPrefix):
				version = line[len(versionPrefix):]
			}
			if label != "" && version != "" {
				return label, version, nil
			}
		}
	}
	return "", "", fmt.Errorf("cannot infer release via /etc/lsb-release, see the --release option")
}
