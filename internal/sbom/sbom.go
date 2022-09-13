package sbom

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/canonical/chisel/internal/control"
)

const sbomFilePath = "/var/lib/dpkg/status"

var sectionKeys = []string{
	"Package",
	"Architecture",
	"Version",
	"Multi-Arch",
	"Priority",
	"Section",
	"Source",
	"Origin",
	"Maintainer",
	"Original-Maintainer",
	"Bugs",
	"Installed-Size",
	"Depends",
	"Recommends",
	"Suggests",
	"Breaks",
	"Replaces",
	"Filename",
	"Size",
	"MD5sum",
	"SHA1",
	"SHA256",
	"SHA512",
	"Homepage",
	"Description",
	"Task",
	"Original-Vcs-Browser",
	"Original-Vcs-Git",
	"Description-md5",
}

type sbomDB struct {
	installedPackages []control.Section
}

var SbomDB = &sbomDB{}

func (db *sbomDB) AddSection(section control.Section) {
	db.installedPackages = append(db.installedPackages, section)
	fmt.Println("Section added:", section)
}

func (db *sbomDB) WriteSections(rootdir string) error {
	dbPath := path.Join(rootdir, sbomFilePath)

	err := os.MkdirAll(filepath.Dir(dbPath), 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	file, err := os.OpenFile(dbPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	for _, pkg := range db.installedPackages {
		for _, key := range sectionKeys {
			val := pkg.Get(key)
			if val == "" {
				continue
			}
			_, err := writer.WriteString(key + ": " + val + "\n")
			if err != nil {
				return err
			}
		}
		writer.WriteString("\n")
	}

	return nil
}