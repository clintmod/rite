package ast

import (
	"go.yaml.in/yaml/v3"

	"github.com/clintmod/rite/errors"
)

// Var represents either a static or dynamic variable.
//
// Export controls whether this variable is propagated to cmd shell
// environments. A nil pointer means "default" — currently equivalent to
// true. Users opt out of export with `export: false` in the map form
// (SPEC §vars/env Unification, non-exported variables section).
type Var struct {
	Value  any
	Live   any
	Sh     *string
	Ref    string
	Dir    string
	Export *bool
}

// Exported reports whether v should be added to a cmd's process environ.
// Unset Export behaves as true (default).
func (v Var) Exported() bool {
	if v.Export == nil {
		return true
	}
	return *v.Export
}

func (v *Var) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.MappingNode:
		// Map forms accept one of four shapes, distinguished by the first key:
		//   sh:    <cmd>            — dynamic var
		//   ref:   <path>           — reference into another var's structure
		//   map:   <any>            — structured value
		//   value: <scalar>         — explicit value form (for pairing with export)
		// plus an optional `export: <bool>` sibling on any shape.
		firstKey := "<none>"
		if len(node.Content) > 0 {
			firstKey = node.Content[0].Value
		}
		switch firstKey {
		case "sh", "ref", "map", "value":
			var m struct {
				Sh     *string
				Ref    string
				Map    any
				Value  any
				Export *bool
			}
			if err := node.Decode(&m); err != nil {
				return errors.NewTaskfileDecodeError(err, node)
			}
			v.Sh = m.Sh
			v.Ref = m.Ref
			v.Export = m.Export
			// value: takes precedence over map: when both are accidentally set.
			if m.Value != nil {
				v.Value = m.Value
			} else {
				v.Value = m.Map
			}
			return nil
		default:
			return errors.NewTaskfileDecodeError(nil, node).WithMessage(`%q is not a valid variable type. Try "sh", "ref", "map", "value" or using a scalar value`, firstKey)
		}
	default:
		var value any
		if err := node.Decode(&value); err != nil {
			return errors.NewTaskfileDecodeError(err, node)
		}
		v.Value = value
		return nil
	}
}
