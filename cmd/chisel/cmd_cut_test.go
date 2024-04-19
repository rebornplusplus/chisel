package main_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/klauspost/compress/zstd"
	. "gopkg.in/check.v1"

	chisel "github.com/canonical/chisel/cmd/chisel"
	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/testutil"
)

var (
	testKey   = testutil.PGPKeys["key1"]
	dbPathExp = regexp.MustCompile(`\s*(\/(?:.*\/)*)\*\*:.*\bgenerate\b:\s*\"?\bmanifest\b\"?`)
)

type cutTest struct {
	summary    string
	release    map[string]string
	arch       string
	slices     []string
	pkgs       map[string][]byte
	filesystem map[string]string
	db         map[string]string
	err        string
}

var cutTests = []cutTest{{
	summary: "Basic cut",
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/file:
						/dir/file-copy:  {copy: /dir/file}
						/other-dir/file: {symlink: ../dir/file}
						/dir/text-file:  {text: data1}
						/dir/foo/bar/:   {make: true, mode: 01777}
				manifest:
					contents:
						/db/**: {generate: manifest}
		`,
	},
	slices: []string{"test-package_myslice", "test-package_manifest"},
	filesystem: map[string]string{
		"/db/":            "dir 0755",
		"/db/chisel.db":   "file 0644 1769a2ce",
		"/dir/":           "dir 0755",
		"/dir/file":       "file 0644 cc55e2ec",
		"/dir/file-copy":  "file 0644 cc55e2ec",
		"/dir/foo/":       "dir 0755",
		"/dir/foo/bar/":   "dir 01777",
		"/dir/text-file":  "file 0644 5b41362b",
		"/other-dir/":     "dir 0755",
		"/other-dir/file": "symlink ../dir/file",
	},
	db: map[string]string{
		"/db/chisel.db": strings.TrimLeft(`
{"jsonwall":"1.0","schema":"1.0","count":16}
{"kind":"content","slice":"test-package_manifest","path":"/db/chisel.db"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/file"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/file-copy"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/foo/bar/"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/text-file"}
{"kind":"content","slice":"test-package_myslice","path":"/other-dir/file"}
{"kind":"package","name":"test-package","version":"test-package_version","sha256":"test-package_hash","arch":"test-package_arch"}
{"kind":"path","path":"/db/chisel.db","mode":"0644","slices":["test-package_manifest"]}
{"kind":"path","path":"/dir/file","mode":"0644","slices":["test-package_myslice"],"sha256":"cc55e2ecf36e40171ded57167c38e1025c99dc8f8bcdd6422368385a977ae1fe","size":14}
{"kind":"path","path":"/dir/file-copy","mode":"0644","slices":["test-package_myslice"],"sha256":"cc55e2ecf36e40171ded57167c38e1025c99dc8f8bcdd6422368385a977ae1fe","size":14}
{"kind":"path","path":"/dir/foo/bar/","mode":"0777","slices":["test-package_myslice"]}
{"kind":"path","path":"/dir/text-file","mode":"0644","slices":["test-package_myslice"],"sha256":"5b41362bc82b7f3d56edc5a306db22105707d01ff4819e26faef9724a2d406c9","size":5}
{"kind":"path","path":"/other-dir/file","mode":"0644","slices":["test-package_myslice"],"link":"../dir/file"}
{"kind":"slice","name":"test-package_manifest"}
{"kind":"slice","name":"test-package_myslice"}
`, "\n"),
	},
}, {
	summary: "All types of paths",
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/file:
						/d?r/s*al/*/:
						/parent/**:
						/dir/file-copy:   {copy: /dir/file}
						/dir/file-copy-2: {copy: /dir/file, mode: 0755}
						/dir/link/file:   {symlink: /dir/file}
						/dir/link/file-2: {symlink: ../file, mode: 0777}
						/dir/text/file:   {text: data1}
						/dir/text/file-2: {text: data2, mode: 0755}
						/dir/text/file-3: {text: data3, mutable: true}
						/dir/text/file-4: {text: data4, until: mutate}
						/dir/text/file-5: {text: "", mode: 0755}
						/dir/text/file-6: {symlink: ./file-3}
						/dir/text/file-7: {text: data7, arch: s390x}
						/dir/all-text:    {text: "", mutable: true}
						/dir/foo/bar/:    {make: true, mode: 01777}
					mutate: |
						content.write("/dir/text/file-3", "foo")
						dir = "/dir/text/"
						data = [ content.read(dir + f) for f in content.list(dir) ]
						content.write("/dir/all-text", "".join(data))
				manifest:
					contents:
						/db/**: {generate: manifest}
		`,
	},
	slices: []string{"test-package_myslice", "test-package_manifest"},
	filesystem: map[string]string{
		"/db/":          "dir 0755",
		"/db/chisel.db": "file 0644 c59e0304",
		"/dir/":         "dir 0755",
		// TODO Note that /dir/all-text has a different hash in db below.
		// This is because mutated info is not being added to db yet.
		// Will be fixed by https://github.com/canonical/chisel/pull/131.
		"/dir/all-text":        "file 0644 8067926c",
		"/dir/file":            "file 0644 cc55e2ec",
		"/dir/file-copy":       "file 0644 cc55e2ec",
		"/dir/file-copy-2":     "file 0644 cc55e2ec",
		"/dir/foo/":            "dir 0755",
		"/dir/foo/bar/":        "dir 01777",
		"/dir/link/":           "dir 0755",
		"/dir/link/file":       "symlink /dir/file",
		"/dir/link/file-2":     "symlink ../file",
		"/dir/several/":        "dir 0755",
		"/dir/several/levels/": "dir 0755",
		"/dir/text/":           "dir 0755",
		"/dir/text/file":       "file 0644 5b41362b",
		"/dir/text/file-2":     "file 0755 d98cf53e",
		"/dir/text/file-3":     "file 0644 2c26b46b",
		// TODO Note that although /dir/text/file-4 is not present in the fs, it
		// is present in db below. This is because "until: mutate" paths have
		// not been filtered yet.
		// Will be fixed by https://github.com/canonical/chisel/pull/131.
		"/dir/text/file-5":         "file 0755 empty",
		"/dir/text/file-6":         "symlink ./file-3",
		"/parent/":                 "dir 01777",
		"/parent/permissions/":     "dir 0764",
		"/parent/permissions/file": "file 0755 722c14b3",
	},
	db: map[string]string{
		"/db/chisel.db": strings.TrimLeft(`
{"jsonwall":"1.0","schema":"1.0","count":40}
{"kind":"content","slice":"test-package_manifest","path":"/db/chisel.db"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/all-text"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/file"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/file-copy"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/file-copy-2"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/foo/bar/"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/link/file"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/link/file-2"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/several/levels/"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/text/file"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/text/file-2"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/text/file-3"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/text/file-4"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/text/file-5"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/text/file-6"}
{"kind":"content","slice":"test-package_myslice","path":"/parent/"}
{"kind":"content","slice":"test-package_myslice","path":"/parent/permissions/"}
{"kind":"content","slice":"test-package_myslice","path":"/parent/permissions/file"}
{"kind":"package","name":"test-package","version":"test-package_version","sha256":"test-package_hash","arch":"test-package_arch"}
{"kind":"path","path":"/db/chisel.db","mode":"0644","slices":["test-package_manifest"]}
{"kind":"path","path":"/dir/all-text","mode":"0644","slices":["test-package_myslice"],"sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}
{"kind":"path","path":"/dir/file","mode":"0644","slices":["test-package_myslice"],"sha256":"cc55e2ecf36e40171ded57167c38e1025c99dc8f8bcdd6422368385a977ae1fe","size":14}
{"kind":"path","path":"/dir/file-copy","mode":"0644","slices":["test-package_myslice"],"sha256":"cc55e2ecf36e40171ded57167c38e1025c99dc8f8bcdd6422368385a977ae1fe","size":14}
{"kind":"path","path":"/dir/file-copy-2","mode":"0644","slices":["test-package_myslice"],"sha256":"cc55e2ecf36e40171ded57167c38e1025c99dc8f8bcdd6422368385a977ae1fe","size":14}
{"kind":"path","path":"/dir/foo/bar/","mode":"0777","slices":["test-package_myslice"]}
{"kind":"path","path":"/dir/link/file","mode":"0644","slices":["test-package_myslice"],"link":"/dir/file"}
{"kind":"path","path":"/dir/link/file-2","mode":"0777","slices":["test-package_myslice"],"link":"../file"}
{"kind":"path","path":"/dir/several/levels/","mode":"0755","slices":["test-package_myslice"]}
{"kind":"path","path":"/dir/text/file","mode":"0644","slices":["test-package_myslice"],"sha256":"5b41362bc82b7f3d56edc5a306db22105707d01ff4819e26faef9724a2d406c9","size":5}
{"kind":"path","path":"/dir/text/file-2","mode":"0755","slices":["test-package_myslice"],"sha256":"d98cf53e0c8b77c14a96358d5b69584225b4bb9026423cbc2f7b0161894c402c","size":5}
{"kind":"path","path":"/dir/text/file-3","mode":"0644","slices":["test-package_myslice"],"sha256":"f60f2d65da046fcaaf8a10bd96b5630104b629e111aff46ce89792e1caa11b18","size":5}
{"kind":"path","path":"/dir/text/file-4","mode":"0644","slices":["test-package_myslice"],"sha256":"02c6edc2ad3e1f2f9a9c8fea18c0702c4d2d753440315037bc7f84ea4bba2542","size":5}
{"kind":"path","path":"/dir/text/file-5","mode":"0755","slices":["test-package_myslice"],"sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}
{"kind":"path","path":"/dir/text/file-6","mode":"0644","slices":["test-package_myslice"],"link":"./file-3"}
{"kind":"path","path":"/parent/","mode":"0777","slices":["test-package_myslice"]}
{"kind":"path","path":"/parent/permissions/","mode":"0764","slices":["test-package_myslice"]}
{"kind":"path","path":"/parent/permissions/file","mode":"0755","slices":["test-package_myslice"],"sha256":"722c14b3fe33f2a36e4e02c0034951d2a6820ad11e0bd633ffa90d09754640cc","size":5}
{"kind":"slice","name":"test-package_manifest"}
{"kind":"slice","name":"test-package_myslice"}
`, "\n"),
	},
}, {
	summary: "Multiple DBs",
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					essential:
						- test-package_manifest
					contents:
						/dir/file:
						/db/**:   {generate: manifest}
				manifest:
					contents:
						/db/**:   {generate: manifest}
						/db-2/**: {generate: manifest}
		`,
	},
	slices: []string{"test-package_myslice"},
	filesystem: map[string]string{
		"/db-2/":          "dir 0755",
		"/db-2/chisel.db": "file 0644 4a51075c",
		"/db/":            "dir 0755",
		"/db/chisel.db":   "file 0644 4a51075c",
		"/dir/":           "dir 0755",
		"/dir/file":       "file 0644 cc55e2ec",
	},
	db: map[string]string{
		"/db/chisel.db": strings.TrimLeft(`
{"jsonwall":"1.0","schema":"1.0","count":11}
{"kind":"content","slice":"test-package_manifest","path":"/db-2/chisel.db"}
{"kind":"content","slice":"test-package_manifest","path":"/db/chisel.db"}
{"kind":"content","slice":"test-package_myslice","path":"/db/chisel.db"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/file"}
{"kind":"package","name":"test-package","version":"test-package_version","sha256":"test-package_hash","arch":"test-package_arch"}
{"kind":"path","path":"/db-2/chisel.db","mode":"0644","slices":["test-package_manifest"]}
{"kind":"path","path":"/db/chisel.db","mode":"0644","slices":["test-package_manifest","test-package_myslice"]}
{"kind":"path","path":"/dir/file","mode":"0644","slices":["test-package_myslice"],"sha256":"cc55e2ecf36e40171ded57167c38e1025c99dc8f8bcdd6422368385a977ae1fe","size":14}
{"kind":"slice","name":"test-package_manifest"}
{"kind":"slice","name":"test-package_myslice"}
`, "\n"),
		"/db-2/chisel.db": strings.TrimLeft(`
{"jsonwall":"1.0","schema":"1.0","count":11}
{"kind":"content","slice":"test-package_manifest","path":"/db-2/chisel.db"}
{"kind":"content","slice":"test-package_manifest","path":"/db/chisel.db"}
{"kind":"content","slice":"test-package_myslice","path":"/db/chisel.db"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/file"}
{"kind":"package","name":"test-package","version":"test-package_version","sha256":"test-package_hash","arch":"test-package_arch"}
{"kind":"path","path":"/db-2/chisel.db","mode":"0644","slices":["test-package_manifest"]}
{"kind":"path","path":"/db/chisel.db","mode":"0644","slices":["test-package_manifest","test-package_myslice"]}
{"kind":"path","path":"/dir/file","mode":"0644","slices":["test-package_myslice"],"sha256":"cc55e2ecf36e40171ded57167c38e1025c99dc8f8bcdd6422368385a977ae1fe","size":14}
{"kind":"slice","name":"test-package_manifest"}
{"kind":"slice","name":"test-package_myslice"}
`, "\n"),
	},
}, {
	summary: "Same file mutated across Multiple packages",
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					essential:
						- test-package_manifest
					contents:
						/dir/file:
						/foo:       {text: foo, mutable: true}
					mutate: |
						content.write("/foo", "test-package")
				manifest:
					contents:
						/db/**:     {generate: manifest}
		`,
		"slices/mydir/other-package.yaml": `
			package: other-package
			slices:
				otherslice:
					contents:
						/foo:       {text: foo, mutable: true}
					mutate: |
						content.write("/foo", "other-package")
		`,
	},
	slices: []string{"test-package_myslice", "other-package_otherslice"},
	pkgs: map[string][]byte{
		"test-package":  testutil.PackageData["test-package"],
		"other-package": testutil.PackageData["other-package"],
	},
	filesystem: map[string]string{
		"/db/":          "dir 0755",
		"/db/chisel.db": "file 0644 3e752568",
		"/dir/":         "dir 0755",
		"/dir/file":     "file 0644 cc55e2ec",
		"/foo":          "file 0644 a46c30a5",
	},
	db: map[string]string{
		"/db/chisel.db": strings.TrimLeft(`
{"jsonwall":"1.0","schema":"1.0","count":12}
{"kind":"content","slice":"other-package_otherslice","path":"/foo"}
{"kind":"content","slice":"test-package_manifest","path":"/db/chisel.db"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/file"}
{"kind":"package","name":"other-package","version":"other-package_version","sha256":"other-package_hash","arch":"other-package_arch"}
{"kind":"package","name":"test-package","version":"test-package_version","sha256":"test-package_hash","arch":"test-package_arch"}
{"kind":"path","path":"/db/chisel.db","mode":"0644","slices":["test-package_manifest"]}
{"kind":"path","path":"/dir/file","mode":"0644","slices":["test-package_myslice"],"sha256":"cc55e2ecf36e40171ded57167c38e1025c99dc8f8bcdd6422368385a977ae1fe","size":14}
{"kind":"path","path":"/foo","mode":"0644","slices":["other-package_otherslice"],"sha256":"2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae","size":3}
{"kind":"slice","name":"other-package_otherslice"}
{"kind":"slice","name":"test-package_manifest"}
{"kind":"slice","name":"test-package_myslice"}
`, "\n"),
	},
}, {
	summary: "No DB if corresponding slice(s) are not selected",
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/file:
						/dir/file-copy:  {copy: /dir/file}
						/other-dir/file: {symlink: ../dir/file}
						/dir/text-file:  {text: data1}
						/dir/foo/bar/:   {make: true, mode: 01777}
				manifest:
					contents:
						/db/**: {generate: manifest}
		`,
	},
	slices: []string{"test-package_myslice"},
	filesystem: map[string]string{
		"/dir/":           "dir 0755",
		"/dir/file":       "file 0644 cc55e2ec",
		"/dir/file-copy":  "file 0644 cc55e2ec",
		"/dir/foo/":       "dir 0755",
		"/dir/foo/bar/":   "dir 01777",
		"/dir/text-file":  "file 0644 5b41362b",
		"/other-dir/":     "dir 0755",
		"/other-dir/file": "symlink ../dir/file",
	},
}, {
	summary: "Non-existing slice ref produces an error",
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/file:
		`,
	},
	slices: []string{"test-package_foo"},
	err:    `slice test-package_foo not found`,
}, {
	summary: "DB path cannot conflict with other paths",
	pkgs: map[string][]byte{
		"test-package":  testutil.PackageData["test-package"],
		"other-package": testutil.PackageData["other-package"],
	},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/file:
		`,
		"slices/mydir/other-package.yaml": `
			package: other-package
			slices:
				manifest:
					contents:
						/dir/**: {generate: manifest}
		`,
	},
	slices: []string{"test-package_myslice", "other-package_manifest"},
	err:    "slices other-package_manifest and test-package_myslice conflict on /dir/\\*\\* and /dir/file",
}, {
	summary: "DB path can conflict with other paths in same package",
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/file:
				manifest:
					contents:
						/dir/**: {generate: manifest}
		`,
	},
	slices: []string{"test-package_myslice", "test-package_manifest"},
	filesystem: map[string]string{
		"/dir/":          "dir 0755",
		"/dir/chisel.db": "file 0644 f07022a7",
		"/dir/file":      "file 0644 cc55e2ec",
	},
	db: map[string]string{
		"/dir/chisel.db": strings.TrimLeft(`
{"jsonwall":"1.0","schema":"1.0","count":8}
{"kind":"content","slice":"test-package_manifest","path":"/dir/chisel.db"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/file"}
{"kind":"package","name":"test-package","version":"test-package_version","sha256":"test-package_hash","arch":"test-package_arch"}
{"kind":"path","path":"/dir/chisel.db","mode":"0644","slices":["test-package_manifest"]}
{"kind":"path","path":"/dir/file","mode":"0644","slices":["test-package_myslice"],"sha256":"cc55e2ecf36e40171ded57167c38e1025c99dc8f8bcdd6422368385a977ae1fe","size":14}
{"kind":"slice","name":"test-package_manifest"}
{"kind":"slice","name":"test-package_myslice"}
`, "\n"),
	},
}}

var defaultChiselYaml = `
	format: v1
	archives:
		ubuntu:
			version: 22.04
			components: [main, universe]
			public-keys: [test-key]
	public-keys:
		test-key:
			id: ` + testKey.ID + `
			armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
`

var archivePackages map[string][]byte

type testArchive struct {
	options archive.Options
	pkgs    map[string][]byte
}

func (a *testArchive) Options() *archive.Options {
	return &a.options
}

func (a *testArchive) Fetch(pkg string) (io.ReadCloser, error) {
	if data, ok := a.pkgs[pkg]; ok {
		return io.NopCloser(bytes.NewBuffer(data)), nil
	}
	return nil, fmt.Errorf("attempted to open %q package", pkg)
}

func (a *testArchive) Exists(pkg string) bool {
	_, ok := a.pkgs[pkg]
	return ok
}

func (a *testArchive) Info(pkg string) (*archive.PackageInfo, error) {
	return &archive.PackageInfo{
		Name:    pkg,
		Version: pkg + "_version",
		Hash:    pkg + "_hash",
		Arch:    pkg + "_arch",
	}, nil
}

func (s *ChiselSuite) TestCut(c *C) {
	for _, test := range cutTests {
		c.Logf("Summary: %s", test.summary)

		if _, ok := test.release["chisel.yaml"]; !ok {
			test.release["chisel.yaml"] = string(defaultChiselYaml)
		}

		if test.pkgs == nil {
			test.pkgs = map[string][]byte{
				"test-package": testutil.PackageData["test-package"],
			}
		}
		archivePackages = test.pkgs

		releaseDir := c.MkDir()
		for path, data := range test.release {
			fpath := filepath.Join(releaseDir, path)
			err := os.MkdirAll(filepath.Dir(fpath), 0755)
			c.Assert(err, IsNil)
			err = os.WriteFile(fpath, testutil.Reindent(data), 0644)
			c.Assert(err, IsNil)
		}

		restore := fakeOpenArchive(openArchive)
		defer restore()

		targetDir := c.MkDir()
		args := []string{"cut", "--release", releaseDir + "/", "--root", targetDir + "/"}
		if test.arch != "" {
			args = append(args, "--arch", test.arch)
		}
		args = append(args, test.slices...)

		extra, err := chisel.Parser().ParseArgs(args)
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err)
			continue
		}
		c.Assert(err, IsNil)
		c.Assert(len(extra), Equals, 0)

		if test.filesystem != nil {
			c.Assert(testutil.TreeDump(targetDir), DeepEquals, test.filesystem)
		}

		if test.db != nil {
			db := make(map[string]string)
			dbPaths := findManifestPaths(test.release)
			for _, path := range dbPaths {
				actualPath := filepath.Clean(filepath.Join(targetDir, path))
				contents, err := extractZSTD(actualPath)
				c.Assert(err, IsNil)
				db[path] = contents
				// fmt.Println(contents)
			}
			c.Assert(db, DeepEquals, test.db)
		}
	}
}

func findManifestPaths(release map[string]string) []string {
	paths := []string{}
	for _, sliceDef := range release {
		for _, line := range strings.Split(sliceDef, "\n") {
			match := dbPathExp.FindStringSubmatch(line)
			if match != nil {
				paths = append(paths, match[1]+"chisel.db")
			}
		}
	}
	return paths
}

func openArchive(opts *archive.Options) (archive.Archive, error) {
	return &testArchive{
		options: *opts,
		pkgs:    archivePackages,
	}, nil
}

func fakeOpenArchive(f func(opts *archive.Options) (archive.Archive, error)) (restore func()) {
	old := chisel.OpenArchive
	chisel.OpenArchive = f
	return func() { chisel.OpenArchive = old }
}

// Extract a zstd-compressed file "src" at path "dest"
func extractZSTD(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	reader, err := zstd.NewReader(file)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	bytes, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
