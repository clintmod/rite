// Command gen-schema emits a JSON Schema for the Ritefile format by
// reflecting on the public AST types in taskfile/ast. Run it with:
//
//	go run ./cmd/gen-schema > website/src/public/schema.json
//
// or via the `gen:schema` task in Ritefile.yml. Two copies are published
// on the docs site:
//
//   - clintmod.github.io/rite/schema.json         — always latest
//   - clintmod.github.io/rite/schema/v3.json      — frozen at the v3 contract
//
// The -id flag sets the schema's $id so both copies self-identify as the
// URL they live at (language servers deref $id and need it to match).
// See #73 for why versioning is independent of the rite binary version.
//
// The reflector does most of the work by walking tagged struct fields.
// For types that reflection can't handle — orderedmap wrappers (Vars,
// Tasks, Includes), types that accept multiple YAML shapes (Cmd, Dep,
// Task, Var, Prompt), and scalar-parsed types (Platform, Glob, *semver.Version) —
// we register custom Mapper overrides below.
//
// Design note: we deliberately do NOT add JSONSchema() methods on AST
// types themselves; that would pull invopop/jsonschema into every
// downstream user of the library. All schema customization lives here.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"reflect"

	"github.com/Masterminds/semver/v3"
	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/clintmod/rite/taskfile/ast"
)

const defaultSchemaID = "https://clintmod.github.io/rite/schema.json"

func main() {
	schemaID := flag.String("id", defaultSchemaID, "value for the schema's $id field (set to the versioned URL when emitting /schema/v3.json)")
	flag.Parse()

	// Main reflector: used to reflect the Ritefile root and all leaf types.
	// Its Mapper intercepts every type that needs special handling, including
	// the polymorphic Cmd/Dep/Task/Var (emitted as oneOf unions).
	r := &jsonschema.Reflector{
		FieldNameTag:               "yaml",
		AllowAdditionalProperties:  true,
		RequiredFromJSONSchemaTags: true,
		Mapper:                     fullTypeMapper,
	}

	// Secondary reflector: same config, but its Mapper only handles the
	// "base" polymorphic types (Version, Vars, Tasks, Includes, Prompt,
	// Platform, Glob, For). It does NOT intercept Cmd/Dep/Task/Var —
	// so reflecting those through this reflector produces their struct-
	// form schemas, which we register as CmdObject/DepObject/TaskObject/
	// Var-object in $defs. The main Mapper's oneOf entries reference those.
	rPlain := &jsonschema.Reflector{
		FieldNameTag:               "yaml",
		AllowAdditionalProperties:  true,
		RequiredFromJSONSchemaTags: true,
		Mapper:                     baseTypeMapper,
	}

	schema := r.Reflect(&ast.Ritefile{})
	schema.ID = jsonschema.ID(*schemaID)
	schema.Title = "Ritefile"
	schema.Description = "JSON Schema for rite Ritefiles. rite is a task runner with first-in-wins variable precedence (shell env > CLI > Ritefile defaults), a hard fork of go-task/task. See https://clintmod.github.io/rite/ for documentation."

	// One-shot plain reflection of Task harvests: Task (struct form),
	// plus all transitive types Task depends on (Cmd struct, Dep struct,
	// For, Prompt, Platform, Requires, Precondition, Defer, Glob, Var,
	// Include indirectly, etc.). Each of those lands in extraSch.Definitions.
	extraSch := rPlain.Reflect(&ast.Task{})

	// Rename the polymorphic types' struct forms to *Object names; the
	// main Mapper's oneOf entries reference those. Precondition is listed
	// here too because it accepts a plain string form (just the sh:
	// expression) as a shorthand, in addition to the full object form.
	renames := map[string]string{
		"Task":         "TaskObject",
		"Cmd":          "CmdObject",
		"Dep":          "DepObject",
		"Precondition": "PreconditionObject",
	}
	for name, def := range extraSch.Definitions {
		target := name
		if renamed, ok := renames[name]; ok {
			target = renamed
		}
		if _, exists := schema.Definitions[target]; !exists {
			schema.Definitions[target] = def
		}
	}

	// Register Task and Var as top-level $defs whose content is the oneOf
	// from the polymorphic mapper. Vars/Tasks reference them by name via
	// AdditionalProperties: refTo("Task") / refTo("Var").
	schema.Definitions["Task"] = polymorphicTypeMapper(reflect.TypeOf(ast.Task{}))
	schema.Definitions["Var"] = polymorphicTypeMapper(reflect.TypeOf(ast.Var{}))
	schema.Definitions["Cmd"] = polymorphicTypeMapper(reflect.TypeOf(ast.Cmd{}))
	schema.Definitions["Dep"] = polymorphicTypeMapper(reflect.TypeOf(ast.Dep{}))
	// Precondition accepts a bare string as shorthand for {sh: "..."}.
	schema.Definitions["Precondition"] = &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{Type: "null", Description: "Skipped — nil entries in preconditions: are ignored at unmarshal time."},
			{Type: "string", Description: "Shell expression; if it exits non-zero, task is aborted with a default message."},
			refTo("PreconditionObject"),
		},
		Description: "A precondition check. String shorthand, null (ignored), or an object with sh: + optional msg:.",
	}
	// VarsWithValidation (used as element type in Requires.vars) accepts a
	// plain string shorthand for a name-only required var.
	schema.Definitions["VarsWithValidation"] = &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{Type: "string", Description: "Variable name; equivalent to {name: 'VAR'}."},
			{
				Type: "object",
				Properties: orderedProps([]namedProp{
					{"name", &jsonschema.Schema{Type: "string", Description: "Variable name to require."}},
					{"enum", &jsonschema.Schema{
						OneOf: []*jsonschema.Schema{
							{Type: "array", Items: &jsonschema.Schema{Type: "string"}, Description: "Literal list of allowed values."},
							{
								Type: "object",
								Properties: orderedProps([]namedProp{
									{"ref", &jsonschema.Schema{Type: "string", Description: "Reference another variable (must resolve to a list)."}},
								}),
								Description: "Dynamic enum — pull allowed values from another variable.",
							},
						},
						Description: "Allowed values. Literal list or {ref: '.VAR'} reference.",
					}},
				}),
				Description: "Full form; name the variable and optionally enumerate allowed values.",
			},
		},
		Description: "A required variable. String shorthand (name only) or object form (name + optional enum).",
	}

	// Reflect leaf types that aren't reachable from Task's tree because
	// their parents (Cmd, Include) go through the mapper instead of plain
	// reflection. Defer is referenced from CmdObject.defer (via fixup below);
	// Include is referenced from Ritefile.includes AdditionalProperties.
	for _, extra := range []any{&ast.Include{}, &ast.Defer{}} {
		extraSch := rPlain.Reflect(extra)
		for name, def := range extraSch.Definitions {
			if _, exists := schema.Definitions[name]; !exists {
				schema.Definitions[name] = def
			}
		}
	}

	// Fix up CmdObject.defer: the outer Cmd struct has `Defer bool` (an
	// internal flag set during unmarshal), but the YAML `defer:` key
	// accepts a string (shell cmd) or an object (task call with vars).
	// Reflection produced `{type: boolean}`; replace it.
	if cmdObj, ok := schema.Definitions["CmdObject"]; ok && cmdObj.Properties != nil {
		deferSchema := &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				{Type: "string", Description: "Shell command to run after the main cmds, on success and failure."},
				refTo("Defer"),
			},
			Description: "Cleanup to run after the task's main cmds complete (on both success and failure paths). String for a shell cmd, object for a task call with vars.",
		}
		cmdObj.Properties.Set("defer", deferSchema)
	}

	// Post-process: clear `required` lists everywhere. Ritefile fields are
	// all optional from a schema perspective. The reflector marks non-pointer
	// fields as required by default; that's correct for struct-literal
	// validation but wrong for a permissive config-file schema.
	clearRequired(schema)
	for _, def := range schema.Definitions {
		clearRequired(def)
	}

	out, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen-schema: marshal: %v\n", err)
		os.Exit(1)
	}
	os.Stdout.Write(out)
	os.Stdout.Write([]byte("\n"))
}

func clearRequired(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	s.Required = nil
	if s.Properties != nil {
		for pair := s.Properties.Oldest(); pair != nil; pair = pair.Next() {
			clearRequired(pair.Value)
		}
	}
	if s.Items != nil {
		clearRequired(s.Items)
	}
	if s.AdditionalProperties != nil {
		clearRequired(s.AdditionalProperties)
	}
	for _, sub := range s.OneOf {
		clearRequired(sub)
	}
	for _, sub := range s.AnyOf {
		clearRequired(sub)
	}
	for _, sub := range s.AllOf {
		clearRequired(sub)
	}
}

// fullTypeMapper is the Mapper used by the primary reflector. It handles
// the base types PLUS the four polymorphic union types (Cmd, Dep, Task, Var).
func fullTypeMapper(t reflect.Type) *jsonschema.Schema {
	if s := polymorphicTypeMapper(t); s != nil {
		return s
	}
	return baseTypeMapper(t)
}

// polymorphicTypeMapper handles Cmd, Dep, Task, Var only. These are split out
// so the plain reflector can reflect them as plain structs for the *Object
// entries in $defs.
func polymorphicTypeMapper(t reflect.Type) *jsonschema.Schema {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t {
	case reflect.TypeOf(ast.Cmd{}):
		return &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				{Type: "null", Description: "Skipped — nil entries in cmds: are ignored at unmarshal time."},
				{Type: "string", Description: "A shell command."},
				refTo("CmdObject"),
			},
			Description: "A shell command string, null (ignored), or an object with cmd/task/defer/for fields.",
		}

	case reflect.TypeOf(ast.Dep{}):
		return &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				{Type: "null", Description: "Skipped — nil entries in deps: are ignored at unmarshal time."},
				{Type: "string", Description: "A task name."},
				refTo("DepObject"),
			},
			Description: "A task name string, null (ignored), or an object with task/vars/for fields.",
		}

	case reflect.TypeOf(ast.Task{}):
		return &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				{Type: "null", Description: "Empty task (no-op)."},
				{Type: "string", Description: "Shortcut: a single shell cmd."},
				{Type: "array", Items: refTo("Cmd"), Description: "Shortcut: a list of cmds (strings or objects)."},
				refTo("TaskObject"),
			},
			Description: "A task definition. Short forms: null (no-op), a single cmd string, or a list of cmds. Full form: an object — see TaskObject.",
		}

	case reflect.TypeOf(ast.Var{}):
		return &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				{Type: "string"},
				{Type: "number"},
				{Type: "boolean"},
				{Type: "array", Description: "Literal list value (e.g. [1, 2, 3] or ['a', 'b'])."},
				{
					Type: "object",
					Properties: orderedProps([]namedProp{
						{"sh", &jsonschema.Schema{Type: "string", Description: "Shell command whose stdout becomes the variable's value."}},
						{"ref", &jsonschema.Schema{Type: "string", Description: "Reference another variable's value."}},
						{"value", anyValue("Explicit value (any type).")},
						{"map", anyValue("Structured map value.")},
						{"export", &jsonschema.Schema{Type: "boolean", Description: "If false, the var is visible in rite templating but NOT exported to the cmd shell environ. Defaults to true."}},
					}),
					Description: "Variable with dynamic value (sh:), cross-reference (ref:), explicit value (value:), or structured map (map:). Add export: false to prevent exporting to subprocess env.",
				},
			},
		}
	}
	return nil
}

// baseTypeMapper handles types that are always polymorphic/scalar-parsed,
// regardless of which reflector is walking them.
func baseTypeMapper(t reflect.Type) *jsonschema.Schema {
	// Dereference pointers so the same mapping covers both Foo and *Foo.
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t {
	case reflect.TypeOf(semver.Version{}):
		return &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				{Type: "string"},
				{Type: "number"},
			},
			Description: "Ritefile format version. Currently only version 3 is supported; may be written as the string \"3\" or the bare number 3.",
			Examples:    []any{"3", 3},
		}

	case reflect.TypeOf(ast.Vars{}):
		return &jsonschema.Schema{
			Type:                 "object",
			AdditionalProperties: refTo("Var"),
			Description:          "Variables (or environment entries). Values can be scalars, dynamic (sh:), or structured maps. Under rite's first-in-wins precedence, entries here are defaults that shell env and CLI args can override.",
		}

	case reflect.TypeOf(ast.Tasks{}):
		return &jsonschema.Schema{
			Type:                 "object",
			AdditionalProperties: refTo("Task"),
			Description:          "Task definitions. Keys are task names (wildcards with '*' are supported).",
		}

	case reflect.TypeOf(ast.Includes{}):
		return &jsonschema.Schema{
			Type:                 "object",
			AdditionalProperties: includeOrStringSchema(),
			Description:          "Included Ritefiles. Values may be a path string or an Include object with taskfile/dir/vars/etc.",
		}

	case reflect.TypeOf(ast.Prompt{}):
		return &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				{Type: "string", Description: "A single confirmation message."},
				{Type: "array", Items: &jsonschema.Schema{Type: "string"}, Description: "A sequence of confirmation messages, each requiring its own y/N."},
			},
			Description: "Warning prompts displayed before the task runs. User must answer 'y' to proceed (override with --yes).",
		}

	// Timestamps parses a scalar — `true`, `false`, or a strftime format string.
	// Without this override, the reflector would walk the struct fields and emit
	// a spurious `{Enabled, Format}` object shape that doesn't match the YAML.
	case reflect.TypeOf(ast.Timestamps{}):
		return &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				{Type: "boolean", Description: "true uses the default ISO 8601 UTC millisecond format; false disables timestamps (useful at task scope to opt out of a global-on setting)."},
				{Type: "string", Description: "A strftime-style format string, e.g. \"[%H:%M:%S]\". See SPEC §Output Timestamps for the supported directive subset."},
			},
			Description: "Prefix every emitted line (cmd output + rite's own log lines) with a timestamp. Settable at the entrypoint, per task, or via --timestamps / RITE_TIMESTAMPS. Precedence: CLI > task > top-level.",
			Examples:    []any{true, "[%Y-%m-%d %H:%M:%S]"},
		}

	// Platform parses a scalar "os", "arch", or "os/arch".
	case reflect.TypeOf(ast.Platform{}):
		return &jsonschema.Schema{
			Type:        "string",
			Description: "OS/arch gate, e.g. 'linux', 'amd64', or 'linux/amd64'. Task/cmd silently skips on non-matching hosts.",
			Examples:    []any{"linux", "darwin/arm64", "windows/amd64"},
		}

	// For accepts a list, a single var name, or a full object with matrix/var/list/sources/as.
	case reflect.TypeOf(ast.For{}):
		matrixAxis := &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				{Type: "array", Description: "Literal list of values along this axis."},
				{
					Type: "object",
					Properties: orderedProps([]namedProp{
						{"ref", &jsonschema.Schema{Type: "string", Description: "Reference another variable (must resolve to a list)."}},
					}),
					Description: "Ref form — pull the axis values from another variable.",
				},
			},
		}
		return &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				{Type: "array", Description: "A literal list of items to iterate."},
				{Type: "string", Description: "A var name to iterate, or the literal 'sources'/'generates' to iterate that glob."},
				{
					Type: "object",
					Properties: orderedProps([]namedProp{
						{"var", &jsonschema.Schema{Type: "string", Description: "Name of a var whose value (split on whitespace or split:) is iterated."}},
						{"list", &jsonschema.Schema{Type: "array", Description: "A literal list of items."}},
						{"matrix", &jsonschema.Schema{Type: "object", AdditionalProperties: matrixAxis, Description: "Named axes for a cartesian product. Each axis is a list or a {ref: '.VAR'} reference."}},
						{"split", &jsonschema.Schema{Type: "string", Description: "Separator for splitting a var's string value. Defaults to whitespace."}},
						{"as", &jsonschema.Schema{Type: "string", Description: "Rename the iteration variable from the default ITEM to a custom name."}},
					}),
				},
			},
			Description: "Iteration spec. Short forms: list of items (string), or a var name / 'sources' / 'generates' (string). Full form: an object with matrix/var/list.",
		}

	// Glob is a string — or an object `{exclude: 'path'}` for negation in sources:/generates:.
	case reflect.TypeOf(ast.Glob{}):
		return &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				{Type: "string", Description: "File glob; prefix with '!' to exclude matches."},
				{
					Type: "object",
					Properties: orderedProps([]namedProp{
						{"exclude", &jsonschema.Schema{Type: "string", Description: "Glob to exclude from the enclosing pattern set."}},
					}),
					Description: "Object form with an exclude: key — equivalent to a '!'-prefixed string.",
				},
			},
			Description: "File glob for sources:/generates:. String form (with optional '!' prefix for negation) or {exclude: 'path'} object.",
			Examples:    []any{"src/**/*.go", "!src/**/*_test.go", map[string]string{"exclude": "./generated.txt"}},
		}
	}

	return nil
}

// --- helpers -----------------------------------------------------------

type namedProp struct {
	name string
	sch  *jsonschema.Schema
}

func orderedProps(pairs []namedProp) *orderedmap.OrderedMap[string, *jsonschema.Schema] {
	out := orderedmap.New[string, *jsonschema.Schema]()
	for _, p := range pairs {
		out.Set(p.name, p.sch)
	}
	return out
}

func refTo(name string) *jsonschema.Schema {
	return &jsonschema.Schema{Ref: "#/$defs/" + name}
}

func anyValue(description string) *jsonschema.Schema {
	return &jsonschema.Schema{Description: description}
}

func includeOrStringSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{Type: "string", Description: "A path string (e.g. './lib')."},
			refTo("Include"),
		},
	}
}
