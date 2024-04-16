package db_test

import (
	"io"
	"os"
	"strings"

	"github.com/canonical/chisel/internal/db"
	"github.com/klauspost/compress/zstd"
	. "gopkg.in/check.v1"
)

type dbTest struct {
	summary    string
	packages   []*db.Package
	slices     []*db.Slice
	paths      []*db.Path
	contents   []*db.Content
	expectedDB string
	err        string
}

var dbTests = []dbTest{{
	summary: "Write Chisel DB",
	packages: []*db.Package{{
		Name:    "mypkg",
		Version: "12ubuntu4.6",
		Digest:  "522d1a2a9a41a86428d20e1a1b619946245ad5a62a348890f1630a6316b69f68",
		Arch:    "amd64",
	}, {
		Name:    "foo",
		Version: "1ubuntu2.9",
		Digest:  "b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c",
		Arch:    "amd64",
	}},
	slices: []*db.Slice{{
		Name: "mypkg_myslice",
	}, {
		Name: "mypkg_otherslice",
	}, {
		Name: "foo_bar",
	}},
	paths: []*db.Path{{
		Path:   "/usr/bin/foo",
		Mode:   "0644",
		Slices: []string{"foo_bar"},
		Hash:   "ebd0b5aaefd98c0f3a56f03d11cb27f858f257eb1206cb8f6264dc72aa8a2947",
		Size:   1234,
	}, {
		Path:   "/bin/foo",
		Mode:   "0644",
		Slices: []string{"foo_bar"},
		Link:   "/usr/bin/foo",
	}, {
		Path:   "/usr/bin/mypkg",
		Mode:   "0775",
		Slices: []string{"mypkg_myslice"},
		Hash:   "c4ce8495a690e25636f83c00b5ee9128f78dcfea24523d2697dbd37114bb967a",
		Size:   49357,
	}, {
		Path:   "/bin/",
		Mode:   "0775",
		Slices: []string{"mypkg_myslice", "foo_bar"},
	}, {
		Path:      "/etc/mypkg.conf",
		Mode:      "0775",
		Slices:    []string{"mypkg_otherslice"},
		Hash:      "c4a49783f9f135204582d2a95f2551c77d8200798fe2c36e774943243985553c",
		FinalHash: "71f28b05f5b0a3af1776ae55d578c16a11f10aef7dd408421c35dac17ca7cbad",
		Size:      489,
	}},
	contents: []*db.Content{{
		Slice: "foo_bar",
		Path:  "/usr/bin/foo",
	}, {
		Slice: "foo_bar",
		Path:  "/bin/foo",
	}, {
		Slice: "foo_bar",
		Path:  "/bin/",
	}, {
		Slice: "mypkg_myslice",
		Path:  "/usr/bin/mypkg",
	}, {
		Slice: "mypkg_myslice",
		Path:  "/bin/",
	}, {
		Slice: "mypkg_otherslice",
		Path:  "/etc/mypkg.conf",
	}},
	expectedDB: strings.TrimLeft(`
{"jsonwall":"1.0","schema":"1.0","count":17}
{"kind":"content","slice":"foo_bar","path":"/bin/"}
{"kind":"content","slice":"foo_bar","path":"/bin/foo"}
{"kind":"content","slice":"foo_bar","path":"/usr/bin/foo"}
{"kind":"content","slice":"mypkg_myslice","path":"/bin/"}
{"kind":"content","slice":"mypkg_myslice","path":"/usr/bin/mypkg"}
{"kind":"content","slice":"mypkg_otherslice","path":"/etc/mypkg.conf"}
{"kind":"package","name":"foo","version":"1ubuntu2.9","sha256":"b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c","arch":"amd64"}
{"kind":"package","name":"mypkg","version":"12ubuntu4.6","sha256":"522d1a2a9a41a86428d20e1a1b619946245ad5a62a348890f1630a6316b69f68","arch":"amd64"}
{"kind":"path","path":"/bin/","mode":"0775","slices":["mypkg_myslice","foo_bar"]}
{"kind":"path","path":"/bin/foo","mode":"0644","slices":["foo_bar"],"link":"/usr/bin/foo"}
{"kind":"path","path":"/etc/mypkg.conf","mode":"0775","slices":["mypkg_otherslice"],"sha256":"c4a49783f9f135204582d2a95f2551c77d8200798fe2c36e774943243985553c","final_sha256":"71f28b05f5b0a3af1776ae55d578c16a11f10aef7dd408421c35dac17ca7cbad","size":489}
{"kind":"path","path":"/usr/bin/foo","mode":"0644","slices":["foo_bar"],"sha256":"ebd0b5aaefd98c0f3a56f03d11cb27f858f257eb1206cb8f6264dc72aa8a2947","size":1234}
{"kind":"path","path":"/usr/bin/mypkg","mode":"0775","slices":["mypkg_myslice"],"sha256":"c4ce8495a690e25636f83c00b5ee9128f78dcfea24523d2697dbd37114bb967a","size":49357}
{"kind":"slice","name":"foo_bar"}
{"kind":"slice","name":"mypkg_myslice"}
{"kind":"slice","name":"mypkg_otherslice"}
`, "\n"),
}}

func (s *S) TestWriteDB(c *C) {
	for _, test := range dbTests {
		c.Logf("Summary: %s", test.summary)

		dir := c.MkDir()
		dbw := db.NewDBWriter(dir)

		var err error
		for _, pkg := range test.packages {
			err = dbw.AddPackage(pkg)
		}
		for _, slice := range test.slices {
			err = dbw.AddSlice(slice)
		}
		for _, path := range test.paths {
			err = dbw.AddPath(path)
		}
		for _, content := range test.contents {
			err = dbw.AddContent(content)
		}
		c.Assert(err, IsNil)

		path, err := dbw.WriteDB()
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err)
		}
		c.Assert(err, IsNil)

		contents, err := extractZSTD(path)
		c.Assert(err, IsNil)
		c.Assert(contents, Equals, test.expectedDB)
	}
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
