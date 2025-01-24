package setup

type YAMLPath = yamlPath

func (r *Release) SetPathPriorities(p map[string]map[string]int) {
	r.pathPriorities = p
}
