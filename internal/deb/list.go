package deb

import (
	"archive/tar"
	"fmt"
	"io"
)

// List returns a list of package paths found in the deb.
func List(pkgReader io.Reader) (paths []string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot list deb contents: %w", err)
		}
	}()

	dataReader, err := getDataReader(pkgReader)
	if err != nil {
		return nil, err
	}
	defer dataReader.Close()

	tarReader := tar.NewReader(dataReader)
	for {
		tarHeader, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		sourcePath := tarHeader.Name
		if len(sourcePath) < 3 || sourcePath[0] != '.' || sourcePath[1] != '/' {
			continue
		}
		sourcePath = sourcePath[1:]
		if sourcePath == "" {
			continue
		}
		paths = append(paths, sourcePath)
	}
	return paths, nil
}
