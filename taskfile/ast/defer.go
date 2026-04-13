package ast

import (
	"go.yaml.in/yaml/v3"

	"github.com/clintmod/rite/errors"
)

type Defer struct {
	Cmd    string `yaml:"defer"`
	Task   string `yaml:"task"`
	Vars   *Vars  `yaml:"vars"`
	Silent bool   `yaml:"silent"`
}

func (d *Defer) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {

	case yaml.ScalarNode:
		var cmd string
		if err := node.Decode(&cmd); err != nil {
			return errors.NewTaskfileDecodeError(err, node)
		}
		d.Cmd = cmd
		return nil

	case yaml.MappingNode:
		var deferStruct struct {
			Defer  string
			Task   string
			Vars   *Vars
			Silent bool
		}
		if err := node.Decode(&deferStruct); err != nil {
			return errors.NewTaskfileDecodeError(err, node)
		}
		d.Cmd = deferStruct.Defer
		d.Task = deferStruct.Task
		d.Vars = deferStruct.Vars
		d.Silent = deferStruct.Silent
		return nil
	}

	return errors.NewTaskfileDecodeError(nil, node).WithTypeMessage("defer")
}
