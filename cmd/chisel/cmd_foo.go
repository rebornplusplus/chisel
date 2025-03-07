package main

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/strdist"
	"github.com/jessevdk/go-flags"
)

type cmdFoo struct {
	Release           string `long:"release"`
	IgnoreSamePackage bool   `short:"i" long:"ignore-same-package"`
	Report            string `short:"o"`
}

var fooDescs = map[string]string{
	"release":             "Chisel release name or directory (e.g. ubuntu-22.04)",
	"ignore-same-package": "Ignore same package conflicts",
	"o":                   "Save report to file",
}

func init() {
	addCommand("foo", "", "", func() flags.Commander { return &cmdFoo{} }, fooDescs, nil)
}

func (cmd *cmdFoo) Execute(args []string) error {
	if len(args) > 0 {
		return ErrExtraArgs
	}

	release, err := obtainRelease(cmd.Release)
	if err != nil {
		return err
	}

	paths := groupPaths(release)
	fmt.Printf("Total paths: %5d\n", len(paths))

	report := checkFoo(paths, cmd.IgnoreSamePackage)
	fmt.Printf("Foo paths:   %5d (%.2f%%)\n",
		len(report.paths), 100*float32(len(report.paths))/float32(len(paths)))

	if cmd.Report != "" {
		f, err := os.Create(cmd.Report)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		_, err = f.WriteString(report.String())
		if err != nil {
			panic(err)
		}
	} else {
		fmt.Print(report.String())
	}

	return nil
}

type pathData struct {
	kind setup.PathKind
	pkg  []*setup.Package
}

// Returns paths grouped by package names.
func groupPaths(r *setup.Release) map[string]*pathData {
	paths := make(map[string]*pathData)
	for _, pkg := range r.Packages {
		for _, s := range pkg.Slices {
			for path, info := range s.Contents {
				data, ok := paths[path]
				if ok {
					if !slices.Contains(data.pkg, pkg) {
						data.pkg = append(data.pkg, pkg)
					}
				} else {
					paths[path] = &pathData{
						kind: info.Kind,
						pkg:  []*setup.Package{pkg},
					}
				}
			}
		}
	}
	return paths
}

type fooData struct {
	path string
	why  string
}

type fooReport struct {
	paths map[string]*fooData
}

func (r *fooReport) String() string {
	var s string

	s += "counts:\n"
	cnt := r.counts()
	keys := make([]string, 0, len(cnt))
	for p := range cnt {
		keys = append(keys, p)
	}
	sort.Strings(keys)
	for _, p := range keys {
		s += fmt.Sprintf("\t%s: %d\n", p, cnt[p])
	}

	s += "paths:\n"
	keys = make([]string, 0, len(r.paths))
	for p := range r.paths {
		keys = append(keys, p)
	}
	sort.Strings(keys)
	for _, p := range keys {
		d := r.paths[p]
		s += fmt.Sprintf("\t%s:\n", p)
		s += fmt.Sprintf("\t\tconflict: %s\n", d.path)
		s += fmt.Sprintf("\t\treason:   %s\n", d.why)
	}

	return s
}

func (r *fooReport) counts() map[string]int {
	c := make(map[string]int)
	for _, d := range r.paths {
		c[d.why]++
	}
	return c
}

func checkFoo(paths map[string]*pathData, ignoreSamePkg bool) *fooReport {
	report := &fooReport{
		paths: make(map[string]*fooData),
	}

	for p, pd := range paths {
		if pd.kind != setup.GlobPath && pd.kind != setup.GeneratePath {
			continue
		}
		for q, qd := range paths {
			if p == q || skipCheck(pd, qd, ignoreSamePkg) {
				continue
			}
			c, why := fooPathsConflict(p, q)
			if c {
				report.paths[p] = &fooData{
					path: q,
					why:  why,
				}
			}
		}
	}

	return report
}

func skipCheck(pd, qd *pathData, skipSame bool) bool {
	if !skipSame {
		return false
	}
	if len(pd.pkg) > 1 || len(qd.pkg) > 1 || pd.pkg[0] != qd.pkg[0] {
		return false
	}
	return true
}

// Check if two paths conflict.
// One of them is a Glob.
func fooPathsConflict(p, q string) (bool, string) {
	// Split the paths into smaller segment.
	ps := strings.Split(p, "/")
	if len(ps) > 0 && ps[0] == "" {
		ps = ps[1:]
	}
	qs := strings.Split(q, "/")
	if len(qs) > 0 && qs[0] == "" {
		qs = qs[1:]
	}
	var swapped bool
	if len(ps) > len(qs) {
		p, q = q, p
		ps, qs = qs, ps
		swapped = true
	}

	validate := func(a string, as []string) {
		// Assert that ** only appears in the tail section, if it does.
		if strings.Contains(a, "**") {
			for i, s := range as {
				back := i == len(as)-1
				has := strings.Contains(s, "**")
				if has && !back {
					panicf("%s: ** should appear at the last segment", a)
				}
			}
		}
	}
	validate(p, ps)
	validate(q, qs)

	hasWild := func(s string) bool {
		return strings.ContainsAny(s, "*?")
	}

	np, nq := len(ps), len(qs)
	eqn := np == nq

	for i := range ps {
		a, b := ps[i], qs[i]

		if i == np-1 {
			if a == "" || (eqn && b == "") {
				// p or q is a directory.
				break
			}
			if strings.Contains(a, "**") {
				// We have already asserted in validate() that ** only appears
				// as the last segment of the path.
				if eqn && !strings.Contains(b, "**") {
					// b (last elem of q) is a file, not a directory.
					continue
				}
				// Match with the remaining segments of q.
				qrem := strings.Join(qs[i:], "/")
				if strdist.GlobPath(a, qrem) {
					if swapped {
						a, qrem = qrem, a
					}
					return true,
						fmt.Sprintf(".../%s and .../%s", a, qrem)
				}
			}
			continue
		}

		if hasWild(a) || hasWild(b) {
			if strdist.GlobPath(a, b) {
				// Since at least one of these segments is a glob and they
				// match, we will need to change the path with the glob.
				if swapped {
					a, b = b, a
				}
				if !hasWild(a) {
					// Change/report only the path that had the deciding glob.
					// If control is here, that means (original) p did not have
					// glob in this segment.
					// Skip this segment and continue on to next segments.
					continue
				}
				return true, fmt.Sprintf(".../%s/ and .../%s/", a, b)
			} else {
				// No reason to check anymore. The paths do not match.
				return false, ""
			}
		}

		if a != b {
			// No reason to check anymore. The paths do not match.
			return false, ""
		}
	}

	return false, ""
}
