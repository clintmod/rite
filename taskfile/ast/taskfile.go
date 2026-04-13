package ast

import (
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"go.yaml.in/yaml/v3"

	"github.com/clintmod/rite/errors"
)

// NamespaceSeparator contains the character that separates namespaces
const NamespaceSeparator = ":"

var V3 = semver.MustParse("3")

// ErrIncludedTaskfilesCantHaveDotenvs is returned when a included Taskfile contains dotenvs
var ErrIncludedTaskfilesCantHaveDotenvs = errors.New("rite: Included Taskfiles can't have dotenv declarations. Please, move the dotenv declaration to the main Taskfile")

// Taskfile is the abstract syntax tree for a Taskfile
type Taskfile struct {
	Location string           `yaml:"-"`
	Version  *semver.Version  `yaml:"version"`
	Output   Output           `yaml:"output"`
	Method   string           `yaml:"method"`
	Includes *Includes        `yaml:"includes"`
	Set      []string         `yaml:"set"`
	Shopt    []string         `yaml:"shopt"`
	Vars     *Vars            `yaml:"vars"`
	Env      *Vars            `yaml:"env"`
	Tasks    *Tasks           `yaml:"tasks"`
	Silent   bool             `yaml:"silent"`
	Dotenv   []string         `yaml:"dotenv"`
	Run      string           `yaml:"run"`
	Interval time.Duration    `yaml:"interval"`
}

// Merge merges the second Taskfile into the first
func (t1 *Taskfile) Merge(t2 *Taskfile, include *Include) error {
	if !t1.Version.Equal(t2.Version) {
		return fmt.Errorf(`rite: Taskfiles versions should match. First is "%s" but second is "%s"`, t1.Version, t2.Version)
	}
	if len(t2.Dotenv) > 0 {
		return ErrIncludedTaskfilesCantHaveDotenvs
	}
	if t2.Output.IsSet() {
		t1.Output = t2.Output
	}
	if t1.Includes == nil {
		t1.Includes = NewIncludes()
	}
	if t1.Vars == nil {
		t1.Vars = NewVars()
	}
	if t1.Env == nil {
		t1.Env = NewVars()
	}
	if t1.Tasks == nil {
		t1.Tasks = NewTasks()
	}
	if t2.Silent {
		for _, t := range t2.Tasks.All(nil) {
			if t.Silent == nil {
				v := true
				t.Silent = &v
			}
		}
	}
	// Env still merges into the parent (vars/env unification is Phase 4 scope
	// per SPEC; env precedence is last-in-wins until then).
	t1.Env.Merge(t2.Env, include)
	// Vars intentionally do NOT merge into the parent: under rite's
	// first-in-wins model (SPEC §Variable Precedence), an included file's
	// top-level `vars:` are tier 6 — include-site vars (tier 5) must beat
	// them, and tier 5 must in turn beat nothing at tier 4. Upstream's
	// flattening merge here would leak t2's vars up into the parent's
	// tier-4 TaskfileVars and pre-empt tier 5 entirely. Pass t2.Vars
	// directly so each task's IncludedTaskfileVars reflects only that
	// task's source file, not a union across all includes.
	return t1.Tasks.Merge(t2.Tasks, include, t2.Vars)
}

func (tf *Taskfile) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.MappingNode:
		var taskfile struct {
			Version  *semver.Version
			Output   Output
			Method   string
			Includes *Includes
			Set      []string
			Shopt    []string
			Vars     *Vars
			Env      *Vars
			Tasks    *Tasks
			Silent   bool
			Dotenv   []string
			Run      string
			Interval time.Duration
		}
		if err := node.Decode(&taskfile); err != nil {
			return errors.NewTaskfileDecodeError(err, node)
		}
		tf.Version = taskfile.Version
		tf.Output = taskfile.Output
		tf.Method = taskfile.Method
		tf.Includes = taskfile.Includes
		tf.Set = taskfile.Set
		tf.Shopt = taskfile.Shopt
		tf.Vars = taskfile.Vars
		tf.Env = taskfile.Env
		tf.Tasks = taskfile.Tasks
		tf.Silent = taskfile.Silent
		tf.Dotenv = taskfile.Dotenv
		tf.Run = taskfile.Run
		tf.Interval = taskfile.Interval
		if tf.Includes == nil {
			tf.Includes = NewIncludes()
		}
		if tf.Vars == nil {
			tf.Vars = NewVars()
		}
		if tf.Env == nil {
			tf.Env = NewVars()
		}
		if tf.Tasks == nil {
			tf.Tasks = NewTasks()
		}
		return nil
	}

	return errors.NewTaskfileDecodeError(nil, node).WithTypeMessage("taskfile")
}
