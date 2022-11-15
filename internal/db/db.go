package db

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/canonical/chisel/internal/setup"
)

type ContentInfo struct {
	Slices  []string    `yaml:"slices,flow"`
	Digest  string      `yaml:"digest,omitempty"`
	Mode    string      `yaml:"mode"`
}

type ReleaseInfo struct {
	Branch   string     `yaml:"branch"`
	Commit   string     `yaml:"commit"`
	Arch     string     `yaml:"arch"`
}

type Database struct {
	Version     uint                         `yaml:"version"`
	Release     ReleaseInfo                  `yaml:"release"`
	Contents    map[string]*ContentInfo      `yaml:"contents"`
	Packages    map[string]string            `yaml:"packages"`

	optional    map[string]bool
}

var ChiselDB *Database

func init() {
	ChiselDB = &Database{
		Version:  1,
		Release:  ReleaseInfo{
			Branch:  "ubuntu-22.04",
			Commit:  "0x1db1c74ebe1e4ee9a888bc90ff7df388e651e018",
			Arch:    "amd64",
		},
		Contents: make(map[string]*ContentInfo),
		Packages: make(map[string]string),

		optional: make(map[string]bool),
	}
}

type RunParams struct {
	PathOwner   map[string][]*setup.Slice
	Optional    map[string]bool
	Packages    map[string]string
}

func computeDigest(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	digest := "0x" + hex.EncodeToString(hasher.Sum(nil))
	return digest, nil
}

func computeMode(filePath string) (string, error) {
	fileInfo, err := os.Lstat(filePath)
	if err != nil {
		return "", err
	}

	mode := strconv.FormatUint(uint64(fileInfo.Mode() & os.ModePerm), 8)
	for len(mode) < 4 {
		mode = "0" + mode
	}
	return mode, nil
}

func prepSlices(slices []*setup.Slice) []string {
	strSlices := make([]string, 0)
	for _, slice := range slices {
		strSlices = append(strSlices, slice.String())
	}
	return strSlices
}

func zipFile(baseDir, zipPath, dbFile string) error {
	archive, err := os.Create(filepath.Join(baseDir, zipPath))
	if err != nil {
		return err
	}
	defer archive.Close()

	zipWriter := zip.NewWriter(archive)
	
	file, err := os.Open(filepath.Join(baseDir, dbFile))
	if err != nil {
		return err
	}
	defer file.Close()

	wrt, err := zipWriter.Create(dbFile)
	if err != nil {
		return err
	}
	_, err = io.Copy(wrt, file)
	if err != nil {
		return err
	}

	zipWriter.Close()

	return os.Remove(filepath.Join(baseDir, dbFile))
}

func (db *Database) prepContents(targetDir string) error {
	removePaths := make([]string, 0)

	for path, info := range db.Contents {
		targetPath := filepath.Join(targetDir, path)
		
		fileInfo, err := os.Lstat(targetPath)
		if err != nil {
			if db.optional[path] {
				removePaths = append(removePaths, path)
				continue
			}
			return err
		}

		info.Mode, err = computeMode(targetPath)
		if err != nil {
			return err
		}

		if fileInfo.IsDir() {
			continue
		}

		info.Digest, err = computeDigest(targetPath)
		if err != nil {
			return err
		}
	}

	for _, path := range removePaths {
		delete(db.Contents, path)
	}
	
	return nil
}

func (db *Database) ParseContentsAndPackages(params *RunParams) error {
	for path, slices := range params.PathOwner {
		db.Contents[path] = &ContentInfo{
			Slices: prepSlices(slices),
		}
	}

	for pkg, version := range params.Packages {
		db.Packages[pkg] = version
	}

	for path, isopt := range params.Optional {
		db.optional[path] = isopt
	}

	return nil
}

func (db *Database) Write(targetDir, zipName, fileName string) error {
	err := db.prepContents(targetDir)
	if err != nil {
		return err
	}

	targetPath := filepath.Join(targetDir, fileName)
	file, err := os.OpenFile(targetPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	enc := yaml.NewEncoder(file)
	err = enc.Encode(*db)
	if err != nil {
		return err
	}

	return zipFile(targetDir, zipName, fileName)
}