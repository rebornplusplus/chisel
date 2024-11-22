package setup

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

// Exported for testing yamlPath.SameContent().
type YAMLPath struct {
	yamlVar *yamlPath
	Path    string
}

func (yp *YAMLPath) parsePath() error {
	dec := yaml.NewDecoder(bytes.NewBuffer([]byte(yp.Path)))
	dec.KnownFields(false)
	yp.yamlVar = &yamlPath{}
	return dec.Decode(yp.yamlVar)
}

func (yp *YAMLPath) SameContent(other *YAMLPath) (bool, error) {
	err := yp.parsePath()
	if err != nil {
		return false, err
	}
	err = other.parsePath()
	if err != nil {
		return false, err
	}
	return yp.yamlVar.SameContent(other.yamlVar), nil
}
