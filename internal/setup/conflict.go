package setup

import (
	"fmt"
	"sort"
	"strings"
)

type Conflict struct {
	// The conflicting path.
	Path string
	// Conflicting path info grouped by slice name (full, e.g. pkg_slice).
	PathInfos map[string]*PathInfo

	// For internal use.
	// Selected slice per package. This slice is one of those which contains the
	// conflicting path in that package.
	pkgSlice map[string]string
}

// ConflictResolution holds info about resolved path conflicts for a particular path.
type ConflictResolution struct {
	Priority map[string]int
}

// ResolveConflict resolves a path conflict by ordering the conflicting
// packages. It takes in a map of PathInfo containing "prefer" relations for
// that path and computes the priority of each package in that relationship.
// For example, if a path in pkgA "prefers" the path in pkgB, pkgB has a higher
// priority for this path than pkgA.
//
// The priority values are non-negative integers. The higher priority package
// should be the one to provide the conflicting path. If two packages have the
// same priority, the path can come from any of them.
//
// This function also validates the graph formed by the "prefer" relations. The
// valid graph is a simple, directed and acyclic graph where each pair of
// vertices (u,v) must meet either of the following two conditions:
//
//  1. There is an acyclic linear "prefer" relationship between u and v.
//     For example, u -> ... -> v is valid where "->" denotes a "prefer"
//     relationship because it is clear that u ultimately prefers v.
//  2. The particular path in u and v are equivalent and each has a non-empty
//     "prefer" value. Here, two paths are equivalent if they have the same
//     properties and are sure to produce the same contents.
//
// Taking these two conditions into account, the overall graph forms like the
// following:
//
//	  A
//	   \
//	    v
//	B -> P -> Q -> ... -> Z
//	    ^
//	   /
//	  C
//
// where there are:
//
//   - A set of vertices (A, B, C) with “indegree” of zero and “outdegree” of
//     one. The vertices all point to the same vertex and this set of vertices
//     are equivalent to each other following the second condition above.
//   - A chain of vertices (P, Q, .., Z) which forms a chain where the
//     indegree of each vertex is one, with the exception of the first vertex in
//     that chain (P), which has an indegree equalling the number of vertices in
//     the first set. Additionally, each of these vertices has an outdegree of
//     one except the last one in the chain (Z) which has a zero outdegree.
//
// Note that it will be a linear graph if the first set has only one vertex.
//
// Priority of the vertices in the first set are 0 and the other vertices have
// incremental priority starting from 1. In the above example, A, B and C all
// will have a priority of 0 and P will have 1, Q has 2 and so on.
//
// If each vertex in the first set has an empty "prefer" value, then the chain
// does not exist and the graph has no edges at all. The graph in that case will
// be just those vertices without any edges. That is valid too. This can be
// considered a special case of the above graph where there is no linear chain.
//
//	  A
//
//	B
//
//	  C
//
// The argument pathInfos is a map from packages to respective *PathInfo for the
// conflicting path. Each package with the conflicting path must have an entry,
// including the one with empty "prefer" value.
func ResolveConflict(c *Conflict) (*ConflictResolution, error) {
	// checkSamePkgConflict must be called first to populate the pkgSlice map.
	err := c.checkSamePkgConflict()
	if err != nil {
		return nil, err
	}
	err = c.checkCycles()
	if err != nil {
		return nil, err
	}
	heads, chain, err := c.splitVertices()
	if err != nil {
		return nil, err
	}
	priority, err := conflictPriority(heads, chain)
	if err != nil {
		return nil, err
	}
	ConflictResolution := &ConflictResolution{
		Priority: priority,
	}
	return ConflictResolution, nil
}

// checkSamePkgConflict returns an error if there are conflicting paths within
// slices of the same package. Paths in a same package conflict if they are the
// same path with different content, or attributes.
func (c *Conflict) checkSamePkgConflict() error {
	c.pkgSlice = make(map[string]string)
	for newSlice, newInfo := range c.PathInfos {
		slice, err := ParseSliceKey(newSlice)
		if err != nil {
			return err
		}
		oldSlice, ok := c.pkgSlice[slice.Package]
		if !ok {
			c.pkgSlice[slice.Package] = newSlice
			continue
		}
		oldInfo := c.PathInfos[oldSlice]
		if !newInfo.SameContent(oldInfo) {
			if oldSlice > newSlice {
				oldSlice, newSlice = newSlice, oldSlice
				oldInfo, newInfo = newInfo, oldInfo
			}
			fmt.Println(c.Path, "info:", oldInfo, oldSlice, newInfo, newSlice)
			return fmt.Errorf("slices %s and %s conflict on %s",
				oldSlice, newSlice, c.Path)
		}
		if oldSlice > newSlice {
			// Keep the lexicographically smallest slice.
			c.pkgSlice[slice.Package] = newSlice
		}
	}
	return nil
}

// checkCycles returns an error if there are any cycles or loops in the conflict
// graph.
func (c *Conflict) checkCycles() error {
	var selfLoop string
	successors := make(map[string][]string, len(c.pkgSlice))
	for pkg, slice := range c.pkgSlice {
		info := c.PathInfos[slice]
		successors[pkg] = make([]string, 0)
		if info.Prefer == "" {
			continue
		}
		if info.Prefer == pkg {
			// This is a loop (self-loop) from the pkg.
			if selfLoop == "" || selfLoop > pkg {
				selfLoop = pkg
			}
		}
		successors[pkg] = append(successors[pkg], info.Prefer)
	}
	if selfLoop != "" {
		return fmt.Errorf("\"prefer\" loop detected for path %s: %s", c.Path, selfLoop)
	}
	components := tarjanSort(successors)
	for _, names := range components {
		if len(names) > 1 {
			return fmt.Errorf("\"prefer\" cycle detected for path %s: %s",
				c.Path, strings.Join(names, ","))
		}
	}
	return nil
}

// splitVertices splits the vertices of an **acyclic** conflict graph into
// "heads" and "chain". The "heads" refer to the first set of vertices in the
// graph with indegree 0. The "chain" refers to the linear graph that exists
// without the heads. The chain is ordered in the direction of edges. In above
// example, [A, B, C] would be the heads and [P, Q, .. Z] is the chain.
func (c *Conflict) splitVertices() (heads, chain []string, err error) {
	pathInfo := func(pkg string) *PathInfo {
		slice := c.pkgSlice[pkg]
		return c.PathInfos[slice]
	}

	nVertices := len(c.pkgSlice)
	indegree := make(map[string]int, nVertices)
	outdegree := make(map[string]int, nVertices)
	for u := range c.pkgSlice {
		indegree[u] = 0
		outdegree[u] = 0
	}
	for u := range c.pkgSlice {
		info := pathInfo(u)
		v := info.Prefer
		if v != "" {
			indegree[v]++
			outdegree[u]++
		}
	}

	// Find the heads.
	for u, d := range indegree {
		if d == 0 {
			heads = append(heads, u)
		}
	}
	if len(heads) == 0 {
		// This should not happen as the graph should be acyclic.
		return nil, nil, fmt.Errorf(`internal error: conflict head not found`)
	}
	// Sort to produce deterministic errors.
	sort.Strings(heads)
	// Validate that the heads are all "equivalent".
	for i, head := range heads {
		if i == 0 {
			continue
		}
		prevHead := heads[0]
		newInfo := pathInfo(head)
		prevInfo := pathInfo(prevHead)
		if !equivalentPaths(newInfo, prevInfo) {
			if prevHead > head {
				prevHead, head = head, prevHead
			}
			return nil, nil, fmt.Errorf("slices %s and %s conflict on %s",
				c.pkgSlice[prevHead], c.pkgSlice[head], c.Path)
		}
	}
	if len(heads) == nVertices {
		// There are no edges in the graph. No path specified "prefer".
		return heads, chain, nil
	}

	// Find the chain.
	var tail string
	for u, d := range outdegree {
		if d != 0 {
			continue
		}
		if tail != "" {
			// Multiple vertices with no "prefer" value specified.
			if u > tail {
				u, tail = tail, u
			}
			return nil, nil, fmt.Errorf("slices %s and %s conflict on %s",
				c.pkgSlice[u], c.pkgSlice[tail], c.Path)
		}
		tail = u
	}
	if tail == "" {
		// This should not happen as we checked for cycles and loops above.
		return nil, nil, fmt.Errorf(`internal error: conflict tail not found`)
	}
	// Walk over the chain, starting with any head.
	for cur := heads[0]; cur != tail && cur != ""; {
		info := pathInfo(cur)
		cur = info.Prefer
		chain = append(chain, cur)
	}
	if len(chain) == 0 {
		return nil, nil, fmt.Errorf(`internal error: conflict chain must not be empty`)
	}
	if chain[len(chain)-1] != tail {
		return nil, nil, fmt.Errorf(`internal error: conflict chain is invalid`)
	}
	if len(heads)+len(chain) != nVertices {
		return nil, nil, fmt.Errorf(`internal error: conflict vertices do not match`)
	}

	return heads, chain, nil
}

// conflictPriority calculates and returns the priority of the vertices in the
// the conflict graph. The vertices in heads are each assigned a priority of 0.
// The vertices in the linear chain are assigned incremental priority, starting
// from 1.
func conflictPriority(heads, chain []string) (priority map[string]int, err error) {
	priority = make(map[string]int)
	for _, u := range heads {
		priority[u] = 0
	}
	for i, u := range chain {
		priority[u] = i + 1
	}
	return priority, nil
}

// equivalentPaths returns true if two paths have the same attributes and they
// are known to produce the exact same content. We can only be sure about the
// contents of artificial paths (text, dir, symlink etc) which do not come from
// packages.
func equivalentPaths(p, q *PathInfo) bool {
	return p.SameContent(q) && p.Kind != CopyPath && p.Kind != GlobPath
}
