package setup

type YAMLPath = yamlPath

func (r *Release) SetConflictRanks(ranks map[string]map[string]int) {
	r.conflictRanks = ranks
}
