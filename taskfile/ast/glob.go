package ast

import (
	"go.yaml.in/yaml/v3"

	"github.com/clintmod/rite/errors"
)

type Glob struct {
	Glob   string `yaml:"-"`
	Negate bool   `yaml:"-"`
}

func (g *Glob) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {

	case yaml.ScalarNode:
		g.Glob = node.Value
		return nil

	case yaml.MappingNode:
		var glob struct {
			Exclude string
		}
		if err := node.Decode(&glob); err != nil {
			return errors.NewRitefileDecodeError(err, node)
		}
		g.Glob = glob.Exclude
		g.Negate = true
		return nil
	}

	return errors.NewRitefileDecodeError(nil, node).WithTypeMessage("glob")
}
