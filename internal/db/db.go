// Package db provides the necessary functionalities to create the Chisel DB.
package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/canonical/chisel/internal/jsonwall"
	"github.com/klauspost/compress/zstd"
)

const dbFile = "chisel.db"
const dbSchema = "1.0"

type DBWriter struct {
	dbPath string
	writer *jsonwall.DBWriter
}

// NewDBWriter returns a db writer that can create new databases. It takes a
// directory path as input where it will write the Chisel DB as chisel.db file.
func NewDBWriter(dir string) *DBWriter {
	if !strings.HasSuffix(dir, "/") {
		dir = dir + "/"
	}
	path := dir + dbFile
	writer := jsonwall.NewDBWriter(&jsonwall.DBWriterOptions{
		Schema: dbSchema,
	})
	return &DBWriter{
		dbPath: path,
		writer: writer,
	}
}

// WriteDB writes all added entries to the Chisel DB and generates the actual
// file. It returns the path of the generated Chisel DB file. The file
// chisel.db is a zstd compressed file.
func (dbw *DBWriter) WriteDB() (path string, err error) {
	path = dbw.dbPath
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}
	defer file.Close()

	w, err := zstd.NewWriter(file)
	if err != nil {
		return "", err
	}
	defer w.Close()

	_, err = dbw.writer.WriteTo(w)
	if err != nil {
		return "", err
	}
	return path, nil
}

type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Digest  string `json:"sha256"`
	Arch    string `json:"arch"`
}

type Slice struct {
	Name string `json:"name"`
}

type Path struct {
	Path      string   `json:"path"`
	Mode      string   `json:"mode"`
	Slices    []string `json:"slices"`
	Hash      string   `json:"sha256,omitempty"`
	FinalHash string   `json:"final_sha256,omitempty"`
	Size      uint64   `json:"size,omitempty"`
	Link      string   `json:"link,omitempty"`
}

type Content struct {
	Slice string `json:"slice"`
	Path  string `json:"path"`
}

type dbBase struct {
	Kind string `json:"kind"`
}

type dbPackage struct {
	dbBase
	Package
}

type dbSlice struct {
	dbBase
	Slice
}

type dbPath struct {
	dbBase
	Path
}

type dbContent struct {
	dbBase
	Content
}

// AddPackage adds a "package"-kind entry to the DB.
func (dbw *DBWriter) AddPackage(pkg *Package) error {
	if pkg == nil {
		return fmt.Errorf("cannot add nil package to DB")
	}
	pkgEntry := &dbPackage{
		dbBase:  dbBase{Kind: "package"},
		Package: *pkg,
	}
	err := dbw.writer.Add(pkgEntry)
	if err != nil {
		return fmt.Errorf("cannot add package %s to DB: %w", pkg.Name, err)
	}
	return nil
}

// AddSlice adds a "slice"-kind entry to the DB.
func (dbw *DBWriter) AddSlice(slice *Slice) error {
	if slice == nil {
		return fmt.Errorf("cannot add nil slice to DB")
	}
	sliceEntry := &dbSlice{
		dbBase: dbBase{Kind: "slice"},
		Slice:  *slice,
	}
	err := dbw.writer.Add(sliceEntry)
	if err != nil {
		return fmt.Errorf("cannot add slice %s to DB: %w", slice.Name, err)
	}
	return nil
}

// AddPath adds a "path"-kind entry to the DB.
func (dbw *DBWriter) AddPath(path *Path) error {
	if path == nil {
		return fmt.Errorf("cannot add nil path to DB")
	}
	pathEntry := &dbPath{
		dbBase: dbBase{Kind: "path"},
		Path:   *path,
	}
	err := dbw.writer.Add(pathEntry)
	if err != nil {
		return fmt.Errorf("cannot add path %s to DB: %w", path.Path, err)
	}
	return nil
}

// AddContent adds a "content"-kind entry to the DB.
func (dbw *DBWriter) AddContent(content *Content) error {
	if content == nil {
		return fmt.Errorf("cannot add nil content to DB")
	}
	contentEntry := &dbContent{
		dbBase:  dbBase{Kind: "content"},
		Content: *content,
	}
	err := dbw.writer.Add(contentEntry)
	if err != nil {
		return fmt.Errorf("cannot add content %v to DB: %w", content, err)
	}
	return nil
}
