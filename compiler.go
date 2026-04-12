package task

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/clintmod/rite/internal/env"
	"github.com/clintmod/rite/internal/execext"
	"github.com/clintmod/rite/internal/filepathext"
	"github.com/clintmod/rite/internal/logger"
	"github.com/clintmod/rite/internal/templater"
	"github.com/clintmod/rite/internal/version"
	"github.com/clintmod/rite/taskfile/ast"
)

type Compiler struct {
	Dir            string
	Entrypoint     string
	UserWorkingDir string

	TaskfileEnv  *ast.Vars
	TaskfileVars *ast.Vars

	Logger *logger.Logger
}

func (c *Compiler) GetTaskfileVariables() (*ast.Vars, error) {
	return c.getVariables(nil, nil, true)
}

func (c *Compiler) GetVariables(t *ast.Task, call *Call) (*ast.Vars, error) {
	return c.getVariables(t, call, true)
}

func (c *Compiler) FastGetVariables(t *ast.Task, call *Call) (*ast.Vars, error) {
	return c.getVariables(t, call, false)
}

// getVariables resolves the full variable set for a task invocation using
// first-in-wins precedence (SPEC §Variable Precedence). Tiers are processed
// highest-priority first and each tier uses set-if-absent: once a key is set
// by a higher-priority tier, no lower tier can overwrite it.
//
// Precedence (highest → lowest):
//  1. Shell environment
//  2. CLI / call.Vars (FOO=bar rite build, --set FOO=bar)
//  3. Entrypoint env block (TaskfileEnv — dotenv files are merged here upstream)
//  4. Entrypoint vars block (TaskfileVars)
//  5. Include-site vars (t.IncludeVars)
//  6. Included-file top-level vars (t.IncludedTaskfileVars)
//  7. Task-scope vars (t.Vars) — defaults only
//  8. Built-in/special vars (RITE_EXE, RITEFILE, TASK, ...)
//
// Built-in vars are made visible to the templater throughout resolution via
// cache.Seed so higher tiers can reference e.g. {{.RITEFILE}} in their
// templates, but they only land in the returned *Vars at the lowest-priority
// merge step — so a user-declared var of the same name would still win.
// (Two-cache templater; see CLAUDE.md Phase 2 notes, option B.)
func (c *Compiler) getVariables(t *ast.Task, call *Call, evaluateShVars bool) (*ast.Vars, error) {
	result := env.GetEnviron()

	specialVars, err := c.getSpecialVars(t, call)
	if err != nil {
		return nil, err
	}

	// Per-resolution dynamic-var cache (SPEC §Dynamic Variables: "cached within
	// a single resolution, not cached across tasks by command string"). A
	// single getVariables call resolves each `sh:` expression at most once;
	// separate calls with different environments each evaluate independently.
	shCache := make(map[string]string)

	// Single persistent cache. cacheMap starts as shell env (from result) and
	// we keep it in sync manually as each tier lands.
	cache := &templater.Cache{Vars: result}
	cache.ResetCache()
	// Built-ins: visible to the templater for every tier's rendering, but not
	// yet part of result — they get added to result last, with set-if-absent,
	// so any user-declared name wins per SPEC §Variable Precedence tier 8.
	specialsAny := make(map[string]any, len(specialVars))
	for k, v := range specialVars {
		specialsAny[k] = v
	}
	cache.Seed(specialsAny)

	setIfAbsent := func(dir string) func(k string, v ast.Var) error {
		return func(k string, v ast.Var) error {
			if _, exists := result.Get(k); exists {
				// A higher-priority tier already set this key.
				return nil
			}
			newVar := templater.ReplaceVar(v, cache)
			// Preserve the Sh field on unevaluated dynamic vars so summary/listing
			// can display them.
			if !evaluateShVars && newVar.Value == nil {
				result.Set(k, ast.Var{Value: "", Sh: newVar.Sh})
				return nil
			}
			if !evaluateShVars {
				result.Set(k, ast.Var{Value: newVar.Value, Sh: newVar.Sh})
				cache.Update(k, newVar.Value)
				return nil
			}
			if err := cache.Err(); err != nil {
				return err
			}
			if newVar.Value != nil || newVar.Sh == nil {
				result.Set(k, ast.Var{Value: newVar.Value})
				cache.Update(k, newVar.Value)
				return nil
			}
			static, err := c.HandleDynamicVar(newVar, dir, env.GetFromVars(result), shCache)
			if err != nil {
				return err
			}
			result.Set(k, ast.Var{Value: static})
			cache.Update(k, static)
			return nil
		}
	}

	rangeFunc := setIfAbsent(c.Dir)

	// Tier 2: CLI / call vars.
	if call != nil {
		for k, v := range call.Vars.All() {
			if err := rangeFunc(k, v); err != nil {
				return nil, err
			}
		}
	}

	// Tier 3: entrypoint env (dotenv merged in upstream).
	for k, v := range c.TaskfileEnv.All() {
		if err := rangeFunc(k, v); err != nil {
			return nil, err
		}
	}

	// Tier 4: entrypoint vars.
	for k, v := range c.TaskfileVars.All() {
		if err := rangeFunc(k, v); err != nil {
			return nil, err
		}
	}

	// Resolve the task's working dir AFTER tiers 1-4 have landed so templates
	// like `dir: '{{.DIRECTORY}}'` can reference entrypoint vars. Upstream
	// could do this earlier because it flattened all vars into one merged
	// map; under first-in-wins we have to honor the tier ordering.
	var taskRangeFunc func(k string, v ast.Var) error
	if t != nil {
		// NOTE(@andreynering): We're manually joining these paths here because
		// this is the raw task, not the compiled one.
		dir := templater.Replace(t.Dir, cache)
		if err := cache.Err(); err != nil {
			return nil, err
		}
		dir = filepathext.SmartJoin(c.Dir, dir)
		taskRangeFunc = setIfAbsent(dir)

		// Tier 5: include-site vars.
		for k, v := range t.IncludeVars.All() {
			if err := rangeFunc(k, v); err != nil {
				return nil, err
			}
		}
		// Tier 6: included file top-level vars.
		for k, v := range t.IncludedTaskfileVars.All() {
			if err := taskRangeFunc(k, v); err != nil {
				return nil, err
			}
		}
		// Tier 7: task-scope vars — defaults only.
		for k, v := range t.Vars.All() {
			if err := taskRangeFunc(k, v); err != nil {
				return nil, err
			}
		}
	}

	// Tier 8: built-ins last, set-if-absent.
	for k, v := range specialVars {
		if _, exists := result.Get(k); exists {
			continue
		}
		result.Set(k, ast.Var{Value: v})
	}

	return result, nil
}

// HandleDynamicVar resolves a `sh:` variable. The optional cache argument is
// the per-resolution dedupe map: within one getVariables / compiledTask
// invocation, an identical `sh:` string resolves once. Across invocations
// (different tasks, different environments) each call evaluates independently,
// which is what SPEC §Dynamic Variables mandates — the upstream
// muDynamicCache keyed globally by command string caused cross-task
// pollution. Pass nil to disable caching entirely.
func (c *Compiler) HandleDynamicVar(v ast.Var, dir string, e []string, cache map[string]string) (string, error) {
	if v.Sh == nil || *v.Sh == "" {
		return "", nil
	}

	if cache != nil {
		if result, ok := cache[*v.Sh]; ok {
			return result, nil
		}
	}

	// NOTE(@andreynering): If a var have a specific dir, use this instead
	if v.Dir != "" {
		dir = v.Dir
	}

	var stdout bytes.Buffer
	opts := &execext.RunCommandOptions{
		Command: *v.Sh,
		Dir:     dir,
		Stdout:  &stdout,
		Stderr:  c.Logger.Stderr,
		Env:     e,
	}
	if err := execext.RunCommand(context.Background(), opts); err != nil {
		return "", fmt.Errorf(`rite: Command "%s" failed: %s`, opts.Command, err)
	}

	result := strings.TrimSuffix(stdout.String(), "\r\n")
	result = strings.TrimSuffix(result, "\n")

	if cache != nil {
		cache[*v.Sh] = result
	}
	c.Logger.VerboseErrf(logger.Magenta, "rite: dynamic variable: %q result: %q\n", *v.Sh, result)

	return result, nil
}

func (c *Compiler) getSpecialVars(t *ast.Task, call *Call) (map[string]string, error) {
	// Use filepath.ToSlash for all paths to ensure consistent forward slashes
	// across platforms. This prevents issues with backslashes being interpreted
	// as escape sequences when paths are used in shell commands on Windows.
	allVars := map[string]string{
		"RITE_EXE":         filepath.ToSlash(os.Args[0]),
		"ROOT_RITEFILE":    filepath.ToSlash(filepathext.SmartJoin(c.Dir, c.Entrypoint)),
		"ROOT_DIR":         filepath.ToSlash(c.Dir),
		"USER_WORKING_DIR": filepath.ToSlash(c.UserWorkingDir),
		"RITE_VERSION":     version.GetVersion(),
	}
	if t != nil {
		allVars["TASK"] = t.Task
		allVars["TASK_DIR"] = filepath.ToSlash(filepathext.SmartJoin(c.Dir, t.Dir))
		allVars["RITEFILE"] = filepath.ToSlash(t.Location.Taskfile)
		allVars["RITEFILE_DIR"] = filepath.ToSlash(filepath.Dir(t.Location.Taskfile))
	} else {
		allVars["TASK"] = ""
		allVars["TASK_DIR"] = ""
		allVars["RITEFILE"] = ""
		allVars["RITEFILE_DIR"] = ""
	}
	if call != nil {
		allVars["ALIAS"] = call.Task
	} else {
		allVars["ALIAS"] = ""
	}

	return allVars, nil
}
