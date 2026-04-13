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
//	DOTENV-ENTRY    task-level dotenv file whose keys collide with
//	                entrypoint env — entrypoint now wins.
//	SECRET-VAR      var name pattern-matches a secret (TOKEN/KEY/SECRET/
//	                PASSWORD/…) and lacks `export: false` — rite auto-exports
//	                `vars:` now, so this would leak to every cmd shell.
//	SCHEMA-URL      `# yaml-language-server: $schema=…taskfile.dev/schema.json`
//	                left in the file — cosmetic pointer at upstream docs.
func Migrate(srcPath string, warn io.Writer) (dstPath string, err error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", err
	}

	dstPath = ritefilePath(srcPath)

	// Warning pass — parse into a narrow shape that captures only what we
	// need to diagnose. We don't reuse ast.Taskfile because it's tuned for
	// execution, not migration, and it eagerly resolves things we want to
	// inspect raw.
	var doc migrateDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		// Don't fail migration on parse error — the user may be converting a
		// taskfile our parser can't handle, and the mechanical rewrite is
		// still useful. Just skip the analysis.
		fmt.Fprintf(warn, "rite migrate: skipping semantic analysis — YAML parse failed: %v\n", err)
	} else {
		doc.emitWarnings(srcPath, warn)
	}

	// Transform pass — mechanical string rewrites only.
	out := rewriteIncludePaths(string(data))
	if hasTaskfileDevSchemaPointer(out) {
		fmt.Fprintf(warn, "rite migrate: SCHEMA-URL %s: `$schema=https://taskfile.dev/schema.json` references upstream docs; rite's schema is not yet published (Phase 5 docs).\n", srcPath)
	}

	if err := os.WriteFile(dstPath, []byte(out), 0o644); err != nil {
		return "", err
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

// migrateDoc captures the minimal shape we need for warning detection.
// Deliberately loose — unknown keys are ignored.
type migrateDoc struct {
	Vars  map[string]yaml.Node   `yaml:"vars"`
	Env   map[string]yaml.Node   `yaml:"env"`
	Tasks map[string]migrateTask `yaml:"tasks"`
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

		// 3: task-level dotenv whose keys collide with entrypoint env.
		if len(t.Dotenv) > 0 && len(d.Env) > 0 {
			dotenvKeys := collectDotenvKeys(t.Dotenv, filepath.Dir(srcPath))
			for _, k := range dotenvKeys {
				if _, dup := d.Env[k]; dup {
					fmt.Fprintf(warn,
						"rite migrate: DOTENV-ENTRY %s task %q: dotenv key %s is also declared at entrypoint env — entrypoint wins in rite.\n",
						srcPath, taskName, k)
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
