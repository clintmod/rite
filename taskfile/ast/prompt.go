package ast

import (
	"go.yaml.in/yaml/v3"

	"github.com/clintmod/rite/errors"
)

type Prompt []string

func (p *Prompt) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var str string
		if err := node.Decode(&str); err != nil {
			return errors.NewRitefileDecodeError(err, node)
		}
		*p = []string{str}
		return nil
	case yaml.SequenceNode:
		var list []string
		if err := node.Decode(&list); err != nil {
			return errors.NewRitefileDecodeError(err, node)
		}
		*p = list
		return nil
	}
	return errors.NewRitefileDecodeError(nil, node).WithTypeMessage("prompt")
}
