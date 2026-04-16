package task

import (
	"fmt"
	"regexp"

	"github.com/clintmod/rite/taskfile/ast"
)

// selfRefRe matches a bash-style self-referential scalar — `${NAME}` or
// `${NAME:-default}` — consuming the whole string (modulo surrounding
// whitespace). Only plain idents are recognized inside the braces; nested
// `${…}` in the default isn't matched, which means `${VAR:-${OTHER}}` slips
// past undetected. That's an acceptable gap for a v1 heuristic.
var selfRefRe = regexp.MustCompile(`^\s*\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^{}]*))?\}\s*$`)

// detectRedundantSelfRef returns (default, true) when value is a whole-string
// self-reference to key. `${KEY}` yields def == ""; `${KEY:-X}` yields
// def == "X". Anything else returns (_, false).
func detectRedundantSelfRef(key, value string) (def string, ok bool) {
	m := selfRefRe.FindStringSubmatch(value)
	if m == nil {
		return "", false
	}
	if m[1] != key {
		return "", false
	}
	return m[2], true
}

// warnRedundantSelfRefs emits one yellow stderr warning per unique
// `KEY: '${KEY[:-default]}'` declaration in the merged Ritefile's static
// vars/env blocks (top-level and per-task). In rite this bash idiom is
// redundant: shell env already overrides Ritefile values per SPEC §Variable
// Precedence, so a plain `KEY: default` has identical behavior. It only
// appears to work inside `cmds:` because the literal `${…}` passes through
// ExpandShell and gets re-parsed by mvdan/sh at cmd-exec time — anywhere
// else in the Ritefile (another var's value, a precondition, a status check)
// the literal leaks through unresolved. See issue #139.
//
// Dynamic (`sh:`) and ref-typed vars are skipped — only literal string
// values are scanned. Warnings are deduplicated on (key, default) so a var
// propagated from one included file to many tasks warns once.
func (e *Executor) warnRedundantSelfRefs() {
	if e.Ritefile == nil || e.Logger == nil {
		return
	}
	seen := map[string]struct{}{}
	report := func(scope, key, def string) {
		sig := key + "\x00" + def
		if _, dup := seen[sig]; dup {
			return
		}
		seen[sig] = struct{}{}
		if def == "" {
			e.Logger.Warnf(
				"rite: %s: %q: '${%s}' is redundant — shell env already overrides Ritefile values; drop the declaration or give it an explicit default (issue #139)\n",
				scope, key, key,
			)
			return
		}
		e.Logger.Warnf(
			"rite: %s: %q: '${%s:-%s}' is redundant — use `%s: %s` instead; shell env will still override it (issue #139)\n",
			scope, key, key, def, key, def,
		)
	}
	scan := func(scope string, vars *ast.Vars) {
		if vars == nil {
			return
		}
		for k, v := range vars.All() {
			if v.Sh != nil || v.Ref != "" {
				continue
			}
			s, ok := v.Value.(string)
			if !ok {
				continue
			}
			if def, match := detectRedundantSelfRef(k, s); match {
				report(scope, k, def)
			}
		}
	}

	scan("vars", e.Ritefile.Vars)
	scan("env", e.Ritefile.Env)
	if e.Ritefile.Tasks != nil {
		for _, t := range e.Ritefile.Tasks.All(nil) {
			scan(fmt.Sprintf("task %q vars", t.Name()), t.Vars)
			scan(fmt.Sprintf("task %q env", t.Name()), t.Env)
		}
	}
}
