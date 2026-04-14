package task

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
	"go.yaml.in/yaml/v3"
)

// Migrate reads a go-task Taskfile at srcPath, writes a converted Ritefile to
// the same directory, and emits a warning for every construct the fork's
// first-in-wins semantics would silently change the meaning of. Warnings go
// to warn; any fatal parse error is returned.
//
// The tool does NOT attempt a deep structural rewrite — upstream's YAML is
// a near-superset of ours and most of it round-trips unchanged. What we DO
// transform on disk:
//
//   - Input file is copied to <dir>/Ritefile<ext>; `Taskfile` substring in
//     include-path literals is rewritten to `Ritefile`.
//   - Any local file referenced from the entrypoint's `includes:` block is
//     recursively migrated the same way — a project structured as
//     `Taskfile.yml` + `.taskfiles/*.Taskfile.yml` ends up as a complete
//     `Ritefile.yml` + `.taskfiles/*.Ritefile.yml` tree in one shot. URL
//     includes and files that don't exist on disk are skipped with a
//     warning; the caller gets a single-file migration in that case.
//
// Everything else flows through verbatim, preserving user comments and
// formatting (we string-substitute rather than re-serialize the AST).
//
// Detected semantic drifts get printed as `rite migrate: <warn-code>: …`
// lines so they're grep-friendly in CI. The five warning classes mirror the
// user-visible breaks documented in Phase 4 of the operating manual:
//
//	OVERRIDE-VAR    task-scope vars: key shadowed by an entrypoint vars: key
//	                — under SPEC tier 7 the task value is now a default only.
//	OVERRIDE-ENV    same, for env: blocks.
//	DOTENV-ENTRY    task-level dotenv file whose keys collide with any
//	                entrypoint-level env source (explicit env:, dotenv:,
//	                or both) — entrypoint now wins.
//	SECRET-VAR      var name pattern-matches a secret (TOKEN/KEY/SECRET/
//	                PASSWORD/…) and lacks `export: false` — rite auto-exports
//	                `vars:` now, so this would leak to every cmd shell.
//	SCHEMA-URL      `# yaml-language-server: $schema=…taskfile.dev/schema.json`
//	                left in the file — cosmetic pointer at upstream docs.
//
// dstPath is the entrypoint's destination — returned even in dry-run mode so
// callers can echo it. Included-file destinations are not surfaced through
// the return value; they appear as `would write …` / `wrote …` lines in
// warn.
func Migrate(srcPath string, warn io.Writer, opts ...MigrateOption) (dstPath string, err error) {
	o := migrateOptions{}
	for _, fn := range opts {
		fn(&o)
	}
	visited := map[string]struct{}{}
	return migrateOne(srcPath, warn, &o, visited, true)
}

// MigrateOption configures Migrate. Functional-options keep the two-arg
// signature callers rely on while leaving room for future knobs.
type MigrateOption func(*migrateOptions)

type migrateOptions struct {
	dryRun bool
}

// WithDryRun returns a MigrateOption that, when true, parses and emits
// warnings but writes no files. Intended for `rite --migrate --dry-run`.
func WithDryRun(enabled bool) MigrateOption {
	return func(o *migrateOptions) { o.dryRun = enabled }
}

// migrateOne performs a single-file migration and recurses into includes.
// visited holds absolute source paths already migrated in this run to
// prevent include cycles (upstream accepts a Taskfile A including B while B
// includes A; only the first visit migrates). isEntrypoint suppresses the
// `wrote …` / `would write …` chatter for the top-level call because the
// CLI already announces the entrypoint.
func migrateOne(srcPath string, warn io.Writer, o *migrateOptions, visited map[string]struct{}, isEntrypoint bool) (dstPath string, err error) {
	abs, err := filepath.Abs(srcPath)
	if err != nil {
		return "", err
	}
	if _, seen := visited[abs]; seen {
		return ritefilePath(srcPath), nil
	}
	visited[abs] = struct{}{}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", err
	}

	dstPath = ritefilePath(srcPath)

	var doc migrateDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		fmt.Fprintf(warn, "rite migrate: skipping semantic analysis — YAML parse failed: %v\n", err)
	} else {
		doc.emitWarnings(srcPath, warn)
	}

	out := rewriteIncludePaths(string(data))
	out = rewriteSpecialVarRefs(out)
	if hasTaskfileDevSchemaPointer(out) {
		fmt.Fprintf(warn, "rite migrate: SCHEMA-URL %s: `$schema=https://taskfile.dev/schema.json` references upstream docs; rite's schema is not yet published (Phase 5 docs).\n", srcPath)
	}

	if o.dryRun {
		if !isEntrypoint {
			fmt.Fprintf(warn, "rite migrate: would write %s\n", dstPath)
		}
	} else {
		if err := os.WriteFile(dstPath, []byte(out), 0o644); err != nil {
			return "", err
		}
		if !isEntrypoint {
			fmt.Fprintf(warn, "rite migrate: wrote %s\n", dstPath)
		}
	}

	// Recurse into includes. Failures are reported, not fatal — a missing
	// included file shouldn't abort the whole migration, since the user may
	// have a partial checkout or an intentionally-optional include.
	srcDir := filepath.Dir(srcPath)
	for namespace, inc := range doc.Includes {
		p := inc.path()
		if p == "" {
			continue
		}
		if isURL(p) {
			fmt.Fprintf(warn, "rite migrate: skipping remote include %q (%s) — rite does not support URL-based includes; migrate the file manually if you need it.\n", namespace, p)
			continue
		}
		resolved := p
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(srcDir, resolved)
		}
		resolved, derr := resolveIncludedTaskfile(resolved)
		if derr != nil {
			if !inc.Optional {
				fmt.Fprintf(warn, "rite migrate: include %q (%s): %v — skipping recursive migration.\n", namespace, p, derr)
			}
			continue
		}
		if _, err := migrateOne(resolved, warn, o, visited, false); err != nil {
			fmt.Fprintf(warn, "rite migrate: include %q (%s): %v — skipping.\n", namespace, resolved, err)
		}
	}

	return dstPath, nil
}

// RitefilePathForTest exports ritefilePath under a test-only name so
// migrate_test.go can table-test filename mapping without promoting the
// helper to the public API.
func RitefilePathForTest(srcPath string) string { return ritefilePath(srcPath) }

// ritefilePath maps a source Taskfile path to its Ritefile counterpart in the
// same directory. Handles both the plain "Taskfile.yml" case and compounded
// forms like "Taskfile.dist.yaml", "Taskfile-inc.yml", etc. A source whose
// basename doesn't contain "Taskfile" gets "Ritefile.yml" as a fallback —
// unusual but ensures we always produce a valid rite filename.
func ritefilePath(srcPath string) string {
	dir, base := filepath.Split(srcPath)
	if strings.Contains(base, "Taskfile") {
		base = strings.Replace(base, "Taskfile", "Ritefile", 1)
	} else if strings.Contains(base, "taskfile") {
		base = strings.Replace(base, "taskfile", "ritefile", 1)
	} else {
		base = "Ritefile.yml"
	}
	return filepath.Join(dir, base)
}

// rewriteIncludePaths swaps `Taskfile` substrings inside the `includes:`
// block's path values. Line-scoped rewrite that walks the YAML as text so
// user comments and formatting survive — a full AST round-trip would throw
// most of that away.
//
// Scope kept deliberately narrow: only lines that are (a) the
// `includes:` header or (b) lines nested under it until a new top-level key
// appears. Anywhere else in the document we leave `Taskfile` alone, because
// the user might be writing about Taskfiles in desc/summary/etc.
func rewriteIncludePaths(s string) string {
	var out strings.Builder
	out.Grow(len(s))
	lines := strings.Split(s, "\n")
	inIncludes := false
	for i, line := range lines {
		if rxIncludesKey.MatchString(line) {
			inIncludes = true
			out.WriteString(line)
			if i < len(lines)-1 {
				out.WriteByte('\n')
			}
			continue
		}
		if inIncludes && rxDedentTopLevel.MatchString(line) {
			inIncludes = false
		}
		if inIncludes && (strings.Contains(line, "Taskfile") || strings.Contains(line, "taskfile")) {
			// Inside an `includes:` block, path-ish values here refer to
			// other ritefiles. Rewrite Taskfile/taskfile.
			line = rewriteInsideIncludesLine(line)
		}
		out.WriteString(line)
		if i < len(lines)-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

var (
	rxIncludesKey    = regexp.MustCompile(`^includes\s*:\s*(?:#.*)?$`)
	rxDedentTopLevel = regexp.MustCompile(`^[A-Za-z]`) // a new top-level YAML key
	rxTaskfileWord   = regexp.MustCompile(`\bTaskfile\b`)
	rxtaskfileWord   = regexp.MustCompile(`\btaskfile\.\w`) // lowercase filename
)

// rewriteInsideIncludesLine swaps Taskfile/taskfile in file-path positions
// while leaving the YAML key `taskfile:` (which is a schema keyword) alone.
// Schema key is preceded by whitespace, followed by `:`. File-path tokens
// appear on the right of a colon or as bare scalar values.
func rewriteInsideIncludesLine(line string) string {
	// Split at the first colon; left = key (leave alone), right = value (rewrite).
	idx := strings.Index(line, ":")
	if idx < 0 {
		// Scalar shortcut form like `  name: Taskfile2.yml` — no separate value.
		return rxTaskfileWord.ReplaceAllString(line, "Ritefile")
	}
	key := line[:idx+1]
	value := line[idx+1:]
	value = rxTaskfileWord.ReplaceAllString(value, "Ritefile")
	// Lowercase Taskfile*.yml as embedded filenames (rare but exists in docs).
	value = rxtaskfileWord.ReplaceAllStringFunc(value, func(m string) string {
		return "ritefile." + m[len(m)-1:]
	})
	return key + value
}

func hasTaskfileDevSchemaPointer(s string) bool {
	return strings.Contains(s, "taskfile.dev/schema")
}

// rewriteSpecialVarRefs rewrites references to go-task's special vars
// into rite's SPEC-preferred aliases:
//
//	.TASK          -> .RITE_NAME
//	.TASK_DIR      -> .RITE_TASK_DIR
//	.TASKFILE      -> .RITEFILE
//	.TASKFILE_DIR  -> .RITEFILE_DIR
//	.ROOT_TASKFILE -> .ROOT_RITEFILE
//	.TASK_VERSION  -> .RITE_VERSION
//
// Both old and new names resolve at runtime (see compiler.getSpecialVars)
// so this is primarily a readability nudge. Historically only .TASK /
// .TASK_DIR had runtime aliases; issue #36 surfaced that the other four
// had neither a rewrite here nor a runtime alias, so migrated Ritefiles
// silently rendered them as the empty string. Both sides were fixed in
// the same change.
//
// Scope: only inside Go-template expressions (`{{ … }}`) to avoid
// mangling prose or user-defined vars that happen to contain `TASK`.
// Order matters: rewrite the long names first so the short-name passes
// don't half-rename them (`.TASK_DIR` and `.TASK_VERSION` before
// `.TASK`; `.TASKFILE_DIR` before `.TASKFILE`; `.ROOT_TASKFILE` before
// `.TASKFILE`). The bracket-class boundaries also guard against it, so
// this is belt-and-braces.
func rewriteSpecialVarRefs(s string) string {
	return rxTemplateExpr.ReplaceAllStringFunc(s, func(expr string) string {
		// Each rewrite runs to a fixed point. The bracket-class boundary
		// approach *consumes* the separator on both sides of a match, so
		// when two refs share a separator (e.g. `{{.TASK .TASK}}` or
		// `{{printf "%s/%s" .TASK_DIR .TASK_DIR}}`) only the first is
		// rewritten on a single pass — the shared separator is eaten by
		// match #1 and unavailable as the lead boundary for match #2.
		// Go's RE2 has no lookahead, so re-scan the output until stable.
		for _, r := range specialVarRewrites {
			for {
				next := r.rx.ReplaceAllString(expr, r.repl)
				if next == expr {
					break
				}
				expr = next
			}
		}
		return expr
	})
}

type specialVarRewrite struct {
	rx   *regexp.Regexp
	repl string
}

var (
	// rxTemplateExpr matches a `{{ ... }}` Go-template expression. Non-greedy
	// so adjacent expressions on one line are matched separately.
	rxTemplateExpr = regexp.MustCompile(`\{\{[\s\S]*?\}\}`)
	// `(?:^|[^.\w])` / `(?:[^.\w]|$)` bracket the match to avoid partial
	// hits like `.MY_TASK_DIR` or `.TASK_NAMESPACE`. Capture groups preserve
	// the bracketing characters. Trailing `[^_\w]` (vs `[^\w]`) excludes
	// underscore so short names don't match the head of longer ones (e.g.
	// `.TASK` wouldn't match the leading part of `.TASK_DIR`; `.TASKFILE`
	// wouldn't match the leading part of `.TASKFILE_DIR`).
	specialVarRewrites = []specialVarRewrite{
		{regexp.MustCompile(`(^|[^.\w])\.TASKFILE_DIR([^\w]|$)`), "${1}.RITEFILE_DIR${2}"},
		{regexp.MustCompile(`(^|[^.\w])\.ROOT_TASKFILE([^\w]|$)`), "${1}.ROOT_RITEFILE${2}"},
		{regexp.MustCompile(`(^|[^.\w])\.TASKFILE([^_\w]|$)`), "${1}.RITEFILE${2}"},
		{regexp.MustCompile(`(^|[^.\w])\.TASK_VERSION([^\w]|$)`), "${1}.RITE_VERSION${2}"},
		{regexp.MustCompile(`(^|[^.\w])\.TASK_DIR([^\w]|$)`), "${1}.RITE_TASK_DIR${2}"},
		{regexp.MustCompile(`(^|[^.\w])\.TASK([^_\w]|$)`), "${1}.RITE_NAME${2}"},
	}
)

// migrateDoc captures the minimal shape we need for warning detection and
// include traversal. Deliberately loose — unknown keys are ignored.
type migrateDoc struct {
	Vars     map[string]yaml.Node      `yaml:"vars"`
	Env      map[string]yaml.Node      `yaml:"env"`
	Dotenv   []string                  `yaml:"dotenv"`
	Tasks    map[string]migrateTask    `yaml:"tasks"`
	Includes map[string]migrateInclude `yaml:"includes"`
}

// migrateInclude captures a single entry in the entrypoint's `includes:`
// block in both of the shapes upstream supports:
//
//	foo: ./path/Taskfile.yml          # scalar shortcut
//	bar: { taskfile: ./path/... }     # mapping
//
// Only fields used for migration-time traversal are decoded.
type migrateInclude struct {
	Taskfile string `yaml:"taskfile"`
	Optional bool   `yaml:"optional"`
}

// UnmarshalYAML accepts either a scalar (treated as the `taskfile:` value) or
// the full mapping form. Anything else is silently ignored — unknown include
// shapes aren't the migrate tool's problem.
func (i *migrateInclude) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		i.Taskfile = node.Value
		return nil
	case yaml.MappingNode:
		type alias migrateInclude
		var a alias
		if err := node.Decode(&a); err != nil {
			return err
		}
		*i = migrateInclude(a)
		return nil
	default:
		return nil
	}
}

// path returns the include's source path, trimmed. An empty return means the
// include is shaped in a way migrate can't follow (e.g. dynamic vars).
func (i migrateInclude) path() string { return strings.TrimSpace(i.Taskfile) }

// isURL reports whether p looks like a remote include. We don't try to
// migrate remote taskfiles — they're not supported by rite at runtime.
func isURL(p string) bool {
	return strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://")
}

// resolveIncludedTaskfile maps an include reference (file or directory) to
// the actual Taskfile on disk. Mirrors go-task's discovery order so anything
// a user can write in `includes:` and have task find at runtime, migrate
// also finds at conversion time.
func resolveIncludedTaskfile(resolved string) (string, error) {
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return resolved, nil
	}
	for _, name := range []string{
		"Taskfile.yml", "Taskfile.yaml",
		"taskfile.yml", "taskfile.yaml",
		"Taskfile.dist.yml", "Taskfile.dist.yaml",
		"taskfile.dist.yml", "taskfile.dist.yaml",
	} {
		cand := filepath.Join(resolved, name)
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
	}
	return "", fmt.Errorf("no Taskfile found in directory %s", resolved)
}

type migrateTask struct {
	Vars   map[string]yaml.Node `yaml:"vars"`
	Env    map[string]yaml.Node `yaml:"env"`
	Dotenv []string             `yaml:"dotenv"`
}

// UnmarshalYAML accepts the three task-definition shapes upstream supports
// (full mapping, bare list of cmds, single-string cmd) and silently drops
// the list/string shorthands — there's nothing to warn about on those since
// they can't declare vars/env/dotenv by definition.
func (t *migrateTask) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	type alias migrateTask
	var a alias
	if err := node.Decode(&a); err != nil {
		return err
	}
	*t = migrateTask(a)
	return nil
}

var secretNameRx = regexp.MustCompile(`(?i)(TOKEN|SECRET|PASSWORD|PASSWD|APIKEY|API_KEY|PRIVATE_KEY|ACCESS_KEY)`)

func (d *migrateDoc) emitWarnings(srcPath string, warn io.Writer) {
	// 1 & 2: task-scope vars/env shadowed by entrypoint (silent tier-7 demotion).
	for taskName, t := range d.Tasks {
		for k := range t.Vars {
			if _, dup := d.Vars[k]; dup {
				fmt.Fprintf(warn,
					"rite migrate: OVERRIDE-VAR %s task %q: vars.%s is also declared at the entrypoint — under SPEC tier 7 the task value is now a default only.\n",
					srcPath, taskName, k)
			}
		}
		for k := range t.Env {
			if _, dup := d.Env[k]; dup {
				fmt.Fprintf(warn,
					"rite migrate: OVERRIDE-ENV %s task %q: env.%s is also declared at the entrypoint — entrypoint wins in rite.\n",
					srcPath, taskName, k)
			}
		}

		// 3: task-level dotenv whose keys collide with any entrypoint-level
		// env source (explicit `env:` map, `dotenv:` files, or both).
		// Upstream go-task let task-level dotenv override entrypoint dotenv;
		// under rite's first-in-wins precedence the entrypoint wins, so the
		// task-level key is silently dropped. We flag all three collision
		// shapes with the source label so users can fix the right site.
		if len(t.Dotenv) > 0 {
			entryKeys := d.entrypointEnvKeys(srcPath)
			if len(entryKeys) > 0 {
				taskDotenvKeys := collectDotenvKeys(t.Dotenv, filepath.Dir(srcPath))
				for _, k := range taskDotenvKeys {
					if source, ok := entryKeys[k]; ok {
						fmt.Fprintf(warn,
							"rite migrate: DOTENV-ENTRY %s task %q: dotenv key %s is also declared at entrypoint %s — entrypoint wins in rite.\n",
							srcPath, taskName, k, source)
					}
				}
			}
		}
	}

	// 4: secret-shaped names under vars: (auto-exported now).
	for k, node := range d.Vars {
		if !secretNameRx.MatchString(k) && !secretNameRx.MatchString(node.Value) {
			continue
		}
		fmt.Fprintf(warn,
			"rite migrate: SECRET-VAR %s vars.%s: name matches a secret pattern and will auto-export to cmd shells in rite. Add `export: false` to keep it Ritefile-internal.\n",
			srcPath, k)
	}
}

// entrypointEnvKeys returns the union of keys declared at entrypoint level
// across the explicit `env:` map and any `dotenv:` files it references.
// Values label the source ("env", "dotenv", or "env+dotenv") so warning
// messages can point the user at the authoritative declaration site.
func (d *migrateDoc) entrypointEnvKeys(srcPath string) map[string]string {
	keys := map[string]string{}
	for k := range d.Env {
		keys[k] = "env"
	}
	if len(d.Dotenv) > 0 {
		for _, k := range collectDotenvKeys(d.Dotenv, filepath.Dir(srcPath)) {
			if existing, ok := keys[k]; ok && existing == "env" {
				keys[k] = "env+dotenv"
			} else if !ok {
				keys[k] = "dotenv"
			}
		}
	}
	return keys
}

// collectDotenvKeys reads each referenced dotenv file relative to srcDir and
// returns the union of their key names. Missing files are silently skipped —
// the migrate warning pass is best-effort.
func collectDotenvKeys(paths []string, srcDir string) []string {
	seen := map[string]struct{}{}
	for _, p := range paths {
		resolved := p
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(srcDir, resolved)
		}
		kv, err := godotenv.Read(resolved)
		if err != nil {
			continue
		}
		for k := range kv {
			seen[k] = struct{}{}
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	return keys
}
