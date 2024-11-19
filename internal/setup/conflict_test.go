package setup_test

import (
	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/setup"
)

type conflictTest struct {
	summary   string
	pathInfos map[string]*setup.PathInfo
	priority  map[string]int
	err       string
}

var conflictTests = []conflictTest{{
	summary: "Linear conflict chain",
	pathInfos: map[string]*setup.PathInfo{
		// a -> b -> c -> d
		"pkg-a_slice": {Prefer: "pkg-b"},
		"pkg-b_slice": {Prefer: "pkg-c"},
		"pkg-c_slice": {Prefer: "pkg-d"},
		"pkg-d_slice": {},
	},
	priority: map[string]int{
		"pkg-a": 0,
		"pkg-b": 1,
		"pkg-c": 2,
		"pkg-d": 3,
	},
}, {
	summary: "Equivalent vertices and linear chain",
	pathInfos: map[string]*setup.PathInfo{
		//	  a
		//	   \
		//	    v
		//	b -> x -> y -> z
		//	    ^
		//	   /
		//	  C
		"pkg-a_slice": {Prefer: "pkg-x", Kind: setup.TextPath},
		"pkg-b_slice": {Prefer: "pkg-x", Kind: setup.TextPath},
		"pkg-c_slice": {Prefer: "pkg-x", Kind: setup.TextPath},
		"pkg-x_slice": {Prefer: "pkg-y"},
		"pkg-y_slice": {Prefer: "pkg-z"},
		"pkg-z_slice": {},
	},
	priority: map[string]int{
		"pkg-a": 0,
		"pkg-b": 0,
		"pkg-c": 0,
		"pkg-x": 1,
		"pkg-y": 2,
		"pkg-z": 3,
	},
}, {
	summary: "Only equivalent vertices",
	pathInfos: map[string]*setup.PathInfo{
		// a      b       c     (no edges)
		"pkg-a_slice": {Kind: setup.DirPath},
		"pkg-b_slice": {Kind: setup.DirPath},
		"pkg-c_slice": {Kind: setup.DirPath},
	},
	priority: map[string]int{
		"pkg-a": 0,
		"pkg-b": 0,
		"pkg-c": 0,
	},
}, {
	summary: "Conflict within same package",
	pathInfos: map[string]*setup.PathInfo{
		"pkg-a_slice1": {Kind: setup.TextPath, Mode: 0644},
		"pkg-a_slice2": {Kind: setup.TextPath, Mode: 0755},
	},
	err: `slices pkg-a_slice1 and pkg-a_slice2 conflict on .*`,
}, {
	summary: "Different prefer from same package",
	pathInfos: map[string]*setup.PathInfo{
		"pkg-a_slice1": {Kind: setup.TextPath, Prefer: "pkg-b"},
		"pkg-a_slice2": {Kind: setup.TextPath, Prefer: "pkg-c"},
	},
	err: `slices pkg-a_slice1 and pkg-a_slice2 conflict on .*`,
}, {
	summary: "Prefer loop to the same package",
	pathInfos: map[string]*setup.PathInfo{
		"pkg-a_slice": {Prefer: "pkg-a"},
	},
	err: `"prefer" loop detected for path .*: pkg-a`,
}, {
	summary: "Multiple prefer self-loops",
	pathInfos: map[string]*setup.PathInfo{
		"pkg-a_slice": {Prefer: "pkg-a"},
		"pkg-b_slice": {Prefer: "pkg-b"},
	},
	err: `"prefer" loop detected for path .*: pkg-a`,
}, {
	summary: "Single prefer cycle",
	pathInfos: map[string]*setup.PathInfo{
		// a -> b -> c
		//      ^   v
		//       \ /
		//        d
		"pkg-a_slice": {Prefer: "pkg-b"},
		"pkg-b_slice": {Prefer: "pkg-c"},
		"pkg-c_slice": {Prefer: "pkg-d"},
		"pkg-d_slice": {Prefer: "pkg-b"},
	},
	err: `"prefer" cycle detected for path .*: pkg-b,pkg-c,pkg-d`,
}, {
	summary: "Multiple prefer cycles",
	pathInfos: map[string]*setup.PathInfo{
		// a -> b -> c        e -> f -> g
		//      ^   v             ^ ^  v
		//       \ /             /   \/
		//        d             h     i
		"pkg-a_slice": {Prefer: "pkg-b", Kind: setup.TextPath},
		"pkg-b_slice": {Prefer: "pkg-c"},
		"pkg-c_slice": {Prefer: "pkg-d"},
		"pkg-d_slice": {Prefer: "pkg-b"},
		"pkg-e_slice": {Prefer: "pkg-f", Kind: setup.TextPath},
		"pkg-f_slice": {Prefer: "pkg-g"},
		"pkg-g_slice": {Prefer: "pkg-i"},
		"pkg-h_slice": {Prefer: "pkg-f", Kind: setup.TextPath},
		"pkg-i_slice": {Prefer: "pkg-f"},
	},
	// Only one is reported.
	err: `"prefer" cycle detected for path .*: pkg-b,pkg-c,pkg-d`,
}, {
	summary: "Disconnected prefer graph",
	pathInfos: map[string]*setup.PathInfo{
		// a -> c       d -> e
		//     ^
		//    /
		//   b
		"pkg-a_slice": {Prefer: "pkg-c", Kind: setup.TextPath},
		"pkg-b_slice": {Prefer: "pkg-c", Kind: setup.TextPath},
		"pkg-c_slice": {},
		"pkg-d_slice": {Prefer: "pkg-e"},
		"pkg-e_slice": {},
	},
	err: `slices pkg-a_slice and pkg-d_slice conflict on .*`,
}, {
	summary: "Empty prefer graph with non-equivalent vertices",
	pathInfos: map[string]*setup.PathInfo{
		// a      b     (no edges)
		"pkg-a_slice": {Kind: setup.TextPath, Info: "a"},
		"pkg-b_slice": {Kind: setup.TextPath, Info: "b"},
	},
	err: `slices pkg-a_slice and pkg-b_slice conflict on .*`,
}, {
	summary: "Non-equivalent vertices with proper linear chain",
	pathInfos: map[string]*setup.PathInfo{
		//	  a
		//	   \
		//	    v
		//	b -> x -> y -> z
		//	    ^
		//	   /
		//	  C
		"pkg-a_slice": {Prefer: "pkg-x", Kind: setup.TextPath},
		"pkg-b_slice": {Prefer: "pkg-x", Kind: setup.SymlinkPath},
		"pkg-c_slice": {Prefer: "pkg-x", Kind: setup.DirPath},
		"pkg-x_slice": {Prefer: "pkg-y"},
		"pkg-y_slice": {Prefer: "pkg-z"},
		"pkg-z_slice": {},
	},
	err: `slices pkg-a_slice and pkg-b_slice conflict on .*`,
}}

func (s *S) TestResolveConflicts(c *C) {
	for _, test := range conflictTests {
		c.Logf("Summary: %s", test.summary)

		// Assume CopyPath for unspecified path Kind.
		for _, pi := range test.pathInfos {
			if pi.Kind == "" {
				pi.Kind = setup.CopyPath
			}
		}

		conflict := &setup.Conflict{
			Path:      "/path",
			PathInfos: test.pathInfos,
		}
		ConflictResolution, err := setup.ResolveConflict(conflict)
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err)
			continue
		}
		c.Assert(err, IsNil)
		c.Assert(ConflictResolution.Priority, DeepEquals, test.priority)
	}
}
