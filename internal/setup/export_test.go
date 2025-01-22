package setup

type YAMLPath = yamlPath

func (r *Release) ConflictRanks() map[string]map[string]int {
	return r.conflictRanks
}
