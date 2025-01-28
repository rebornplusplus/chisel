package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/openpgp/packet"

	"github.com/canonical/chisel/internal/apacheutil"
	"github.com/canonical/chisel/internal/strdist"
)

// Release is a collection of package slices targeting a particular
// distribution version.
type Release struct {
	Path     string
	Packages map[string]*Package
	Archives map[string]*Archive

	// pathPriorities will store package priorities if there is a 'prefer'
	// relationship. Otherwise, it will be nil.
	// For each path, packages have numerical priorities. Given a selection of
	// packages, the path should be extracted from the one with the highest
	// priority.
	pathPriorities map[string]map[string]int
}

// Archive is the location from which binary packages are obtained.
type Archive struct {
	Name       string
	Version    string
	Suites     []string
	Components []string
	Priority   int
	Pro        string
	PubKeys    []*packet.PublicKey
}

// Package holds a collection of slices that represent parts of themselves.
type Package struct {
	Name    string
	Path    string
	Archive string
	Slices  map[string]*Slice
}

// Slice holds the details about a package slice.
type Slice struct {
	Package   string
	Name      string
	Essential []SliceKey
	Contents  map[string]PathInfo
	Scripts   SliceScripts
}

type SliceScripts struct {
	Mutate string
}

type PathKind string

const (
	DirPath      PathKind = "dir"
	CopyPath     PathKind = "copy"
	GlobPath     PathKind = "glob"
	TextPath     PathKind = "text"
	SymlinkPath  PathKind = "symlink"
	GeneratePath PathKind = "generate"

	// TODO Maybe in the future, for binary support.
	//Base64Path PathKind = "base64"
)

type PathUntil string

const (
	UntilNone   PathUntil = ""
	UntilMutate PathUntil = "mutate"
)

type GenerateKind string

const (
	GenerateNone     GenerateKind = ""
	GenerateManifest GenerateKind = "manifest"
)

type PathInfo struct {
	Kind PathKind
	Info string
	Mode uint

	Mutable  bool
	Until    PathUntil
	Arch     []string
	Generate GenerateKind
	Prefer   string
}

// SameContent returns whether the path has the same content properties as some
// other path. In other words, the resulting file/dir entry is the same. The
// Mutable flag must also match, as that's a common agreement that the actual
// content is not well defined upfront.
func (pi *PathInfo) SameContent(other *PathInfo) bool {
	return (pi.Kind == other.Kind &&
		pi.Info == other.Info &&
		pi.Mode == other.Mode &&
		pi.Mutable == other.Mutable &&
		pi.Generate == other.Generate)
}

type SliceKey = apacheutil.SliceKey

func ParseSliceKey(sliceKey string) (SliceKey, error) {
	return apacheutil.ParseSliceKey(sliceKey)
}

func (s *Slice) String() string { return s.Package + "_" + s.Name }

// Selection holds the required configuration to create a Build for a selection
// of slices from a Release. It's still an abstract proposal in the sense that
// the real information coming from packages is still unknown, so referenced
// paths could potentially be missing, for example.
type Selection struct {
	Release *Release
	Slices  []*Slice
}

// SelectPackage returns true if path should be extracted from pkg.
func (s *Selection) SelectPackage(path, pkg string) bool {
	// If the path has no prefer relationships then it is always selected.
	priorities, ok := s.Release.pathPriorities[path]
	if !ok {
		return true
	}

	// If there is a prefer relationship, we choose the package with the highest
	// priority among the selection.
	pkgPriority, ok := priorities[pkg]
	if !ok {
		return false
	}
	// TODO possible optimization: we could cache the results because they only
	// depend on the selection.
	for _, slice := range s.Slices {
		if p, ok := priorities[slice.Package]; ok {
			if p > pkgPriority {
				return false
			}
		}
	}
	return true
}

func ReadRelease(dir string) (*Release, error) {
	logDir := dir
	if strings.Contains(dir, "/.cache/") {
		logDir = filepath.Base(dir)
	}
	logf("Processing %s release...", logDir)

	release := &Release{
		Path:     dir,
		Packages: make(map[string]*Package),
	}

	release, err := readRelease(dir)
	if err != nil {
		return nil, err
	}

	err = release.validate()
	if err != nil {
		return nil, err
	}
	return release, nil
}

func reportConflict(old, new *Slice, path string) error {
	if old.Package > new.Package || old.Package == new.Package && old.Name > new.Name {
		old, new = new, old
	}
	return fmt.Errorf("slices %s and %s conflict on %s", old, new, path)
}

func (r *Release) validate() error {
	keys := []SliceKey(nil)

	// Check for info conflicts and prepare for following checks. A conflict
	// means that two slices attempt to extract different files or directories
	// to the same location.
	// Conflict validation is done without downloading packages which means that
	// if we are extracting content from different packages to the same location
	// we cannot be sure that it will be the same. On the contrary, content
	// extracted from the same package will never conflict because it is
	// guaranteed to be the same.
	// The above also means that generated content (e.g. text files, directories
	// with make:true) will always conflict with extracted content, because we
	// cannot validate that they are the same without downloading the package.
	globs := make(map[string]*Slice)
	// Used for conflict resolution using 'prefer'. See preferGraph.
	graphs := make(map[string]*preferGraph)

	// Iterate on a stable package order.
	var pkgNames []string
	for _, pkg := range r.Packages {
		pkgNames = append(pkgNames, pkg.Name)
	}
	slices.Sort(pkgNames)
	for _, pkgName := range pkgNames {
		pkg := r.Packages[pkgName]
		for _, new := range pkg.Slices {
			keys = append(keys, SliceKey{pkg.Name, new.Name})
			for newPath, newInfo := range new.Contents {
				if g, ok := graphs[newPath]; ok {
					if s, ok := g.visited[new.Package]; ok {
						// If the package was already visited we only need to
						// check that the new path provides the same content as
						// the recorded one and they have the same prefer
						// relationship.
						sInfo := s.Contents[newPath]
						if !newInfo.SameContent(&sInfo) || newInfo.Prefer != sInfo.Prefer {
							return reportConflict(s, new, newPath)
						}
						continue
					}

					if newInfo.Prefer != "" {
						if len(g.visited) > 1 && !g.isLinear() {
							// Since there are already two or more package paths
							// with no 'prefer' specified, the graph must be
							// disconnected.
							return reportConflict(g.head, new, newPath)
						}
					} else {
						// Since there are no prefer relationships, compare with
						// any slice seen before.
						headInfo := g.head.Contents[newPath]
						if headInfo.Prefer != "" || !newInfo.SameContent(&headInfo) ||
							((newInfo.Kind == CopyPath || newInfo.Kind == GlobPath) && new.Package != g.head.Package) {
							return reportConflict(g.head, new, newPath)
						}
					}
					if err := g.visit(new); err != nil {
						return err
					}
					continue
				}

				if newInfo.Kind == GeneratePath || newInfo.Kind == GlobPath {
					globs[newPath] = new
				}
				g := &preferGraph{
					path:    newPath,
					release: r,
					visited: make(map[string]*Slice),
				}
				graphs[newPath] = g
				if err := g.visit(new); err != nil {
					return err
				}
			}
		}
	}
	for path, g := range graphs {
		if !g.isLinear() {
			continue
		}
		if r.pathPriorities == nil {
			r.pathPriorities = make(map[string]map[string]int)
		}
		r.pathPriorities[path] = make(map[string]int)
		counter := 0
		for cur := g.head.Package; cur != ""; cur = g.next(cur) {
			counter++
			r.pathPriorities[path][cur] = counter
		}
	}

	// Check for glob and generate conflicts.
	for oldPath, old := range globs {
		oldInfo := old.Contents[oldPath]
		for newPath, g := range graphs {
			if oldPath == newPath {
				// Identical globs have been filtered earlier. This must be the
				// exact same entry.
				continue
			}
			if !strdist.GlobPath(newPath, oldPath) {
				continue
			}
			for _, new := range g.visited {
				// It is okay to check only one slice per packages because the
				// content has been validated to be the same earlier.
				newInfo := new.Contents[newPath]
				if oldInfo.Kind == GlobPath && (newInfo.Kind == GlobPath || newInfo.Kind == CopyPath) {
					if new.Package == old.Package {
						continue
					}
				}
				if (old.Package > new.Package) || (old.Package == new.Package && old.Name > new.Name) ||
					(old.Package == new.Package && old.Name == new.Name && oldPath > newPath) {
					old, new = new, old
					oldPath, newPath = newPath, oldPath
				}
				return fmt.Errorf("slices %s and %s conflict on %s and %s", old, new, oldPath, newPath)
			}
		}
	}

	// Check for cycles.
	_, err := order(r.Packages, keys)
	if err != nil {
		return err
	}

	// Check for archive priority conflicts.
	priorities := make(map[int]*Archive)
	for _, archive := range r.Archives {
		if old, ok := priorities[archive.Priority]; ok {
			if old.Name > archive.Name {
				archive, old = old, archive
			}
			return fmt.Errorf("chisel.yaml: archives %q and %q have the same priority value of %d", old.Name, archive.Name, archive.Priority)
		}
		priorities[archive.Priority] = archive
	}

	// Check that archives pinned in packages are defined.
	for _, pkg := range r.Packages {
		if pkg.Archive == "" {
			continue
		}
		if _, ok := r.Archives[pkg.Archive]; !ok {
			return fmt.Errorf("%s: package refers to undefined archive %q", pkg.Path, pkg.Archive)
		}
	}

	return nil
}

func order(pkgs map[string]*Package, keys []SliceKey) ([]SliceKey, error) {

	// Preprocess the list to improve error messages.
	for _, key := range keys {
		if pkg, ok := pkgs[key.Package]; !ok {
			return nil, fmt.Errorf("slices of package %q not found", key.Package)
		} else if _, ok := pkg.Slices[key.Slice]; !ok {
			return nil, fmt.Errorf("slice %s not found", key)
		}
	}

	// Collect all relevant package slices.
	successors := map[string][]string{}
	pending := append([]SliceKey(nil), keys...)

	seen := make(map[SliceKey]bool)
	for i := 0; i < len(pending); i++ {
		key := pending[i]
		if seen[key] {
			continue
		}
		seen[key] = true
		pkg := pkgs[key.Package]
		slice := pkg.Slices[key.Slice]
		fqslice := slice.String()
		predecessors := successors[fqslice]
		for _, req := range slice.Essential {
			fqreq := req.String()
			if reqpkg, ok := pkgs[req.Package]; !ok || reqpkg.Slices[req.Slice] == nil {
				return nil, fmt.Errorf("%s requires %s, but slice is missing", fqslice, fqreq)
			}
			predecessors = append(predecessors, fqreq)
		}
		successors[fqslice] = predecessors
		pending = append(pending, slice.Essential...)
	}

	// Sort them up.
	var order []SliceKey
	for _, names := range tarjanSort(successors) {
		if len(names) > 1 {
			return nil, fmt.Errorf("essential loop detected: %s", strings.Join(names, ", "))
		}
		name := names[0]
		dot := strings.IndexByte(name, '_')
		order = append(order, SliceKey{name[:dot], name[dot+1:]})
	}

	return order, nil
}

func readRelease(baseDir string) (*Release, error) {
	baseDir = filepath.Clean(baseDir)
	filePath := filepath.Join(baseDir, "chisel.yaml")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read release definition: %s", err)
	}
	release, err := parseRelease(baseDir, filePath, data)
	if err != nil {
		return nil, err
	}
	err = readSlices(release, baseDir, filepath.Join(baseDir, "slices"))
	if err != nil {
		return nil, err
	}
	return release, err
}

func readSlices(release *Release, baseDir, dirName string) error {
	entries, err := os.ReadDir(dirName)
	if err != nil {
		return fmt.Errorf("cannot read %s%c directory", stripBase(baseDir, dirName), filepath.Separator)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			err := readSlices(release, baseDir, filepath.Join(dirName, entry.Name()))
			if err != nil {
				return err
			}
			continue
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		match := apacheutil.FnameExp.FindStringSubmatch(entry.Name())
		if match == nil {
			return fmt.Errorf("invalid slice definition filename: %q", entry.Name())
		}

		pkgName := match[1]
		pkgPath := filepath.Join(dirName, entry.Name())
		if pkg, ok := release.Packages[pkgName]; ok {
			return fmt.Errorf("package %q slices defined more than once: %s and %s\")", pkgName, pkg.Path, pkgPath)
		}
		data, err := os.ReadFile(pkgPath)
		if err != nil {
			// Errors from package os generally include the path.
			return fmt.Errorf("cannot read slice definition file: %v", err)
		}

		pkg, err := parsePackage(baseDir, pkgName, stripBase(baseDir, pkgPath), data)
		if err != nil {
			return err
		}

		release.Packages[pkg.Name] = pkg
	}
	return nil
}

func stripBase(baseDir, path string) string {
	// Paths must be clean for this to work correctly.
	return strings.TrimPrefix(path, baseDir+string(filepath.Separator))
}

func Select(release *Release, slices []SliceKey) (*Selection, error) {
	logf("Selecting slices...")

	selection := &Selection{
		Release: release,
	}

	sorted, err := order(release.Packages, slices)
	if err != nil {
		return nil, err
	}
	selection.Slices = make([]*Slice, len(sorted))
	for i, key := range sorted {
		selection.Slices[i] = release.Packages[key.Package].Slices[key.Slice]
	}

	for _, new := range selection.Slices {
		for newPath, newInfo := range new.Contents {
			// An invalid "generate" value should only throw an error if that
			// particular slice is selected. Hence, the check is here.
			switch newInfo.Generate {
			case GenerateNone, GenerateManifest:
			default:
				return nil, fmt.Errorf("slice %s has invalid 'generate' for path %s: %q",
					new, newPath, newInfo.Generate)
			}
		}
	}

	return selection, nil
}

// findPath returns a package slice which contains the path. It may return any
// slice if there are multiple.
func findPath(r *Release, path, pkg string) (*Slice, error) {
	for _, s := range r.Packages[pkg].Slices {
		if _, ok := s.Contents[path]; ok {
			return s, nil
		}
	}
	return nil, fmt.Errorf("package %s does not have path %s", pkg, path)
}

// preferGraph stores the 'prefer' relationship representation for a path. There
// are only two valid configurations of a graph - either all the nodes are
// disconnected and they produce the same content, or there is a linear order of
// 'prefer' relationship. In any other case, the graph functions returns an
// error.
type preferGraph struct {
	path    string
	release *Release
	// The initial node (represented by a slice of the package) of the chain. If
	// the graph is not linear, then head may be any node.
	head    *Slice
	visited map[string]*Slice
}

// isLinear returns true if the 'prefer' relations form a linear graph.
func (g *preferGraph) isLinear() bool {
	return g.head != nil && g.head.Contents[g.path].Prefer != ""
}

// Returns the next node (package name) in the 'prefer' chain.
func (g *preferGraph) next(pkg string) string {
	return g.visited[pkg].Contents[g.path].Prefer
}

// findCycle returns the nodes in a cycle if a cycle has been detected,
// otherwise it returns an error.
func (g *preferGraph) findCycle(visited map[string]*Slice, start string) ([]string, error) {
	var cycle []string
	for u := start; ; u = visited[u].Contents[g.path].Prefer {
		if u == "" {
			return nil, fmt.Errorf("internal error: expected a cycle")
		}
		if len(cycle) > 0 && u == cycle[0] {
			break
		}
		cycle = append(cycle, u)
	}
	var minIdx int
	for i, v := range cycle {
		if v < cycle[minIdx] {
			minIdx = i
		}
	}
	cycle = append(cycle[minIdx:], cycle[:minIdx]...)
	return cycle, nil
}

// visit iterates over the 'prefer' relations, starting from src.
func (g *preferGraph) visit(src *Slice) error {
	// A variant of Topological sorting using DFS is used in this function.
	// See https://en.wikipedia.org/wiki/Topological_sorting#Depth-first_search.
	orderSlices := func(a, b *Slice) (*Slice, *Slice) {
		if a.Package > b.Package || (a.Package == b.Package && a.Name > b.Name) {
			a, b = b, a
		}
		return a, b
	}
	tempVisited := make(map[string]*Slice)

	// prev is used for error reporting.
	var prev string
	cur := src.Package
	for {
		if _, ok := tempVisited[cur]; ok {
			cycle, err := g.findCycle(tempVisited, cur)
			if err != nil {
				return err
			}
			return fmt.Errorf("slice %s path %s has a 'prefer' cycle: %s",
				tempVisited[cycle[0]], g.path, strings.Join(cycle, ", "))
		}
		if _, ok := g.visited[cur]; ok {
			// This has been previously visited, thus this chain stops here.
			if cur != g.head.Package {
				// The chain should have stopped at g.head, but since it does
				// not, it means that it is not a linear 'prefer' graph, rather
				// a "Y" shaped one.
				a, b := orderSlices(src, g.head)
				return fmt.Errorf("slices %s and %s conflict on path %s: "+
					"path has 'prefer' but there is no valid linear order", a, b, g.path)
			}
			break
		}

		var s *Slice
		if cur == src.Package {
			s = src
		} else {
			var err error
			s, err = findPath(g.release, g.path, cur)
			if err != nil {
				if prev != "" {
					return fmt.Errorf("slice %s path %s has an invalid 'prefer' %q: %s",
						tempVisited[prev], g.path, cur, err)
				}
				return err
			}
		}
		tempVisited[cur] = s

		next := s.Contents[g.path].Prefer
		if next == "" {
			// The tail of the chain is found. This chain stops here.
			if g.isLinear() {
				// A tail has been found whereas this chain should have stopped
				// at g.head.Package. Thus, the 'prefer' graph must have more
				// than one components i.e. disconnected.
				// Note: this is how we deviate from DFS described above.
				a, b := orderSlices(src, g.head)
				return fmt.Errorf("slices %s and %s conflict on path %s: "+
					"path has 'prefer' but there is no valid linear order", a, b, g.path)
			}
			break
		}
		if _, ok := g.release.Packages[next]; !ok {
			return fmt.Errorf("slice %s path %s 'prefer' refers to undefined package %q", s, g.path, next)
		}
		prev = cur
		cur = next
	}

	// Update the head and mark the nodes.
	g.head = src
	for p, s := range tempVisited {
		g.visited[p] = s
	}
	return nil
}
