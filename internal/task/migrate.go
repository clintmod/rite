package task

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
	"go.yaml.in/yaml/v3"
	"mvdan.cc/sh/v3/syntax"
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
//   - The entrypoint is written as `<dir>/Ritefile<ext>` (e.g.
//     `Taskfile.yml` → `Ritefile.yml`, `Taskfile.dist.yaml` →
//     `Ritefile.dist.yaml`).
//   - Any local file referenced from any walked file's `includes:` block is
//     recursively migrated. Each included source gets a per-source
//     destination name derived from its basename (`ci/a.yml` →
//     `ci/a.Ritefile.yml`; `.taskfiles/android.Taskfile.yml` →
//     `.taskfiles/android.Ritefile.yml`). The root is the only file that
//     claims the canonical `Ritefile<ext>` name.
//   - Every `includes:` entry whose target resolves to a file we migrated is
//     rewritten to point at the new destination. URL includes and files
//     that don't exist on disk are skipped with a warning and left unchanged
//     in the rewritten YAML.
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
//	TEMPLATE-KEPT   a Go-template expression (`{{if}}`, `{{range}}`, sprig
//	                helper, etc.) that has no equivalent ${VAR} form was
//	                left verbatim — review manually.
//	SELFREF-CMD     a cmd shelled out to the `task` binary (e.g. `task lint`,
//	                `cd sub && task build`, `$(task --version)`); the head of
//	                each affected CallExpr is rewritten to `rite` so the
//	                migrated Ritefile drives itself instead of go-task.
//
// The yaml-language-server `$schema=` directive pointing at taskfile.dev is
// rewritten to rite's hosted schema in place; the previous SCHEMA-URL
// warning class is gone — migrate fixes the pointer instead of complaining
// about it.
//
// dstPath is the entrypoint's destination — returned even in dry-run mode so
// callers can echo it. Included-file destinations are not surfaced through
// the return value; they appear as `would write …` / `wrote …` lines in
// warn.
//
// Implementation is two-phase: Phase 1 walks the include tree and builds a
// map from absolute source path to absolute destination path, choosing a
// unique destination per source. Phase 2 emits each file using that map to
// rewrite include targets in one coordinated pass. A prior single-pass
// version (pre-#76) picked `<dir>/Ritefile.yml` for every walked source,
// which meant N sibling includes all wrote to the same destination path
// (silent clobber) and the root's `includes:` block kept pointing at the
// original `.yml` filenames (no rewrite possible without per-source
// identity).
func Migrate(srcPath string, warn io.Writer, opts ...MigrateOption) (dstPath string, err error) {
	o := migrateOptions{}
	for _, fn := range opts {
		fn(&o)
	}

	rootAbs, err := filepath.Abs(srcPath)
	if err != nil {
		return "", err
	}

	// Phase 1: discover the tree.
	pathMap := map[string]string{rootAbs: mustAbs(ritefilePath(srcPath))}
	order := []string{rootAbs}
	collectIncludeTree(rootAbs, pathMap, &order, warn)

	// Phase 2: emit each file with coordinated include-target rewrites.
	for _, absSrc := range order {
		isEntrypoint := absSrc == rootAbs
		if err := emitMigratedFile(absSrc, pathMap, o, warn, isEntrypoint); err != nil {
			return "", err
		}
	}
	return ritefilePath(srcPath), nil
}

// mustAbs resolves to an absolute path or returns the input unchanged on
// error. Absolute-ification failure is unusual (filepath.Abs falls back to
// cwd lookup); if it does fail the downstream WriteFile will surface a
// clearer error than we could construct here.
func mustAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

// collectIncludeTree populates pathMap (absSrc → absDst) and order (BFS
// insertion order, root first) for every local Taskfile reachable from
// srcAbs via `includes:` edges. Failures — parse errors, missing required
// includes, cycles — drop the offending child from the tree but don't abort;
// the emit phase will report them. Cycles are detected by the pathMap
// presence check.
func collectIncludeTree(srcAbs string, pathMap map[string]string, order *[]string, warn io.Writer) {
	data, err := os.ReadFile(srcAbs)
	if err != nil {
		return
	}
	var doc migrateDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return
	}
	srcDir := filepath.Dir(srcAbs)
	for _, inc := range doc.Includes {
		p := inc.path()
		if p == "" || isURL(p) {
			continue
		}
		resolved := p
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(srcDir, resolved)
		}
		resolved, derr := resolveIncludedTaskfile(resolved)
		if derr != nil {
			continue
		}
		childAbs, err := filepath.Abs(resolved)
		if err != nil {
			continue
		}
		if _, seen := pathMap[childAbs]; seen {
			continue
		}
		pathMap[childAbs] = mustAbs(includedRitefilePath(childAbs))
		*order = append(*order, childAbs)
		collectIncludeTree(childAbs, pathMap, order, warn)
	}
}

// emitMigratedFile reads the source at absSrc, applies all rewrites, and
// writes (or announces, under dry-run) to pathMap[absSrc]. The entrypoint's
// `wrote …` / `would write …` line is suppressed because the CLI announces
// the entrypoint destination separately; nested files get the announcement
// so the user can see the tree walk.
func emitMigratedFile(absSrc string, pathMap map[string]string, o migrateOptions, warn io.Writer, isEntrypoint bool) error {
	data, err := os.ReadFile(absSrc)
	if err != nil {
		return err
	}

	var doc migrateDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		fmt.Fprintf(warn, "rite migrate: skipping semantic analysis — YAML parse failed: %v\n", err)
	} else {
		doc.emitWarnings(absSrc, warn)
	}

	// Warn once per missing/remote include — only on the parent that
	// declares it, and only in the emit phase so we don't double-announce.
	srcDir := filepath.Dir(absSrc)
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
		if _, derr := resolveIncludedTaskfile(resolved); derr != nil {
			if !inc.Optional {
				fmt.Fprintf(warn, "rite migrate: include %q (%s): %v — skipping recursive migration.\n", namespace, p, derr)
			}
		}
	}

	rewrites := computeIncludeRewrites(doc, srcDir, pathMap)
	out := rewriteIncludePathsWithMap(string(data), rewrites)
	out = rewriteSpecialVarRefs(out)
	out = rewriteSchemaPointer(out)
	if !o.keepGoTemplates {
		out = modernizeTemplates(out, absSrc, warn)
	}
	out = rewriteSelfRefCmds(out, absSrc, warn)

	dst := pathMap[absSrc]
	if o.dryRun {
		if !isEntrypoint {
			fmt.Fprintf(warn, "rite migrate: would write %s\n", dst)
		}
		return nil
	}
	if err := os.WriteFile(dst, []byte(out), 0o644); err != nil {
		return err
	}
	if !isEntrypoint {
		fmt.Fprintf(warn, "rite migrate: wrote %s\n", dst)
	}
	return nil
}

// MigrateOption configures Migrate. Functional-options keep the two-arg
// signature callers rely on while leaving room for future knobs.
type MigrateOption func(*migrateOptions)

type migrateOptions struct {
	dryRun          bool
	keepGoTemplates bool
}

// WithDryRun returns a MigrateOption that, when true, parses and emits
// warnings but writes no files. Intended for `rite --migrate --dry-run`.
func WithDryRun(enabled bool) MigrateOption {
	return func(o *migrateOptions) { o.dryRun = enabled }
}

// WithKeepGoTemplates returns a MigrateOption that, when true, suppresses
// the Go-template → `${VAR}` modernization pass. Intended for
// `rite migrate --keep-go-templates` when a user deliberately wants the
// old template surface preserved verbatim.
func WithKeepGoTemplates(enabled bool) MigrateOption {
	return func(o *migrateOptions) { o.keepGoTemplates = enabled }
}

// RitefilePathForTest exports ritefilePath under a test-only name so
// migrate_test.go can table-test filename mapping without promoting the
// helper to the public API.
func RitefilePathForTest(srcPath string) string { return ritefilePath(srcPath) }

// IncludedRitefilePathForTest exports includedRitefilePath for the same
// reason as RitefilePathForTest.
func IncludedRitefilePathForTest(srcPath string) string { return includedRitefilePath(srcPath) }

// ritefilePath maps the ENTRYPOINT source path to its Ritefile counterpart.
// Handles the plain "Taskfile.yml" case and compounded forms like
// "Taskfile.dist.yaml", "Taskfile-inc.yml", etc. A source whose basename
// doesn't contain "Taskfile" gets "Ritefile.yml" as a fallback — unusual
// for a top-level invocation, but preserves the canonical discovery name.
// This is only called for the entrypoint; nested includes go through
// includedRitefilePath so two sibling includes can't collide on the same
// destination filename.
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

// includedRitefilePath maps a NESTED include source to its migrated
// destination. Unlike the entrypoint case, every included source needs a
// distinct filename so N sibling includes don't all collapse onto one
// `<dir>/Ritefile.yml` (the #76 regression from the original walk). Rule:
//
//   - Basename contains "Taskfile" → swap to "Ritefile" (e.g.
//     "android.Taskfile.yml" → "android.Ritefile.yml"). The source basename
//     already carries its own identity, so a simple swap preserves it.
//   - Basename contains "taskfile" (lowercase) → same swap on lowercase.
//   - Basename has no Taskfile/taskfile marker (e.g. "a.yml", "android.yml") →
//     insert ".Ritefile" before the extension: "a.yml" → "a.Ritefile.yml".
//     This preserves the stem as disambiguation and the compound
//     "*.Ritefile.*" suffix signals the file is a migration artifact.
//   - Basename with no extension → append ".Ritefile.yml".
//
// Files produced by this function are discoverable by rite only if the user
// renames them later, but that's by design: the include-target rewrite in
// emitMigratedFile updates the parent's `includes:` block to point directly
// at the new path, so discovery isn't needed in the normal case.
func includedRitefilePath(srcPath string) string {
	dir, base := filepath.Split(srcPath)
	if strings.Contains(base, "Taskfile") {
		return filepath.Join(dir, strings.Replace(base, "Taskfile", "Ritefile", 1))
	}
	if strings.Contains(base, "taskfile") {
		return filepath.Join(dir, strings.Replace(base, "taskfile", "ritefile", 1))
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if ext == "" {
		return filepath.Join(dir, stem+".Ritefile.yml")
	}
	return filepath.Join(dir, stem+".Ritefile"+ext)
}

// includeRewrite records a path literal to rewrite in an `includes:` block.
// oldLiteral is the exact string the user wrote for the include target
// (e.g. `./lib/Taskfile.yml`); newLiteral is the relative path to the
// migrated destination from the parent file's directory. Both are kept as
// strings so the rewrite stays at the line level and user comments/
// formatting survive unchanged — a full YAML AST round-trip would lose
// both.
type includeRewrite struct {
	oldLiteral string
	newLiteral string
}

// computeIncludeRewrites turns the parent doc's `includes:` entries into a
// list of (oldLiteral, newLiteral) pairs, using pathMap to find the
// destination each local include was migrated to. Entries we skipped during
// the tree walk (URLs, missing files) produce no rewrite — they stay
// unchanged in the emitted YAML so the user sees their original reference
// and can decide what to do manually.
func computeIncludeRewrites(doc migrateDoc, parentDir string, pathMap map[string]string) []includeRewrite {
	var rewrites []includeRewrite
	for _, inc := range doc.Includes {
		p := inc.path()
		if p == "" || isURL(p) {
			continue
		}
		resolved := p
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(parentDir, resolved)
		}
		resolved, derr := resolveIncludedTaskfile(resolved)
		if derr != nil {
			continue
		}
		childAbs, err := filepath.Abs(resolved)
		if err != nil {
			continue
		}
		newAbs, ok := pathMap[childAbs]
		if !ok {
			continue
		}
		newRel, err := filepath.Rel(parentDir, newAbs)
		if err != nil {
			continue
		}
		newRel = filepath.ToSlash(newRel)
		// Preserve the leading `./` convention if the original had one;
		// filepath.Rel strips it.
		if strings.HasPrefix(p, "./") && !strings.HasPrefix(newRel, ".") {
			newRel = "./" + newRel
		}
		rewrites = append(rewrites, includeRewrite{oldLiteral: p, newLiteral: newRel})
	}
	return rewrites
}

// rewriteIncludePathsWithMap performs include-target substitution inside the
// `includes:` block. Line-scoped so user comments and formatting survive
// unchanged. Scope stays narrow: only lines inside `includes:` (from the
// header until a line begins with a new top-level YAML key) are candidates.
//
// Rewrites land in two tiers:
//
//  1. Exact-match against `rewrites` (built from the migrated tree): if the
//     parent's include line contains a literal path we know corresponds to a
//     file we migrated, substitute the relative path to the new destination.
//     This is the correct case — the YAML ends up pointing at a real
//     migrated file.
//
//  2. Fallback `Taskfile`/`taskfile` substring swap on any remaining line
//     (same line-level rewrite the pre-#76 tool did). This keeps migrations
//     useful even when the included file isn't on disk at migrate time
//     (partial checkout, intentionally missing include, etc.) — the user
//     gets a filename that matches rite's discovery rules even though we
//     couldn't recurse into it.
//
// Each rewrite is applied once per line via strings.Replace(_, _, _, 1), so
// two include entries sharing the same literal path would each get a
// separate substitution on the correct line. One include value per line is
// the YAML convention; mixed-shape forms (scalar vs mapping) still keep the
// value on its own line.
func rewriteIncludePathsWithMap(s string, rewrites []includeRewrite) string {
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
		if inIncludes {
			rewritten := false
			for _, r := range rewrites {
				if strings.Contains(line, r.oldLiteral) {
					line = strings.Replace(line, r.oldLiteral, r.newLiteral, 1)
					rewritten = true
					break
				}
			}
			if !rewritten && (strings.Contains(line, "Taskfile") || strings.Contains(line, "taskfile")) {
				line = rewriteInsideIncludesLineFallback(line)
			}
		}
		out.WriteString(line)
		if i < len(lines)-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

// rewriteInsideIncludesLineFallback swaps Taskfile/taskfile in file-path
// positions while leaving the YAML key `taskfile:` (a schema keyword) alone.
// Used only when the exact-path rewriter couldn't match — i.e. the include
// target wasn't reachable from disk during the tree walk.
func rewriteInsideIncludesLineFallback(line string) string {
	idx := strings.Index(line, ":")
	if idx < 0 {
		// Scalar shortcut form like `  - Taskfile2.yml` — no separate value.
		return rxTaskfileWord.ReplaceAllString(line, "Ritefile")
	}
	key := line[:idx+1]
	value := line[idx+1:]
	value = rxTaskfileWord.ReplaceAllString(value, "Ritefile")
	value = rxtaskfileWord.ReplaceAllStringFunc(value, func(m string) string {
		return "ritefile." + m[len(m)-1:]
	})
	return key + value
}

var (
	rxIncludesKey    = regexp.MustCompile(`^includes\s*:\s*(?:#.*)?$`)
	rxDedentTopLevel = regexp.MustCompile(`^[A-Za-z]`) // a new top-level YAML key
	rxTaskfileWord   = regexp.MustCompile(`\bTaskfile\b`)
	rxtaskfileWord   = regexp.MustCompile(`\btaskfile\.\w`) // lowercase filename
)

// rxSchemaPointer matches a yaml-language-server `$schema=` directive whose
// URL contains `taskfile.dev`, tolerating:
//   - indentation before `#`
//   - optional whitespace around `yaml-language-server:` and after `=`
//   - optional single- or double-quoted URL
//   - any path after the host (schema.json, schema/v3.json, etc.)
//
// Group 1 is everything up to and including the opening quote (if any);
// group 2 is the URL; group 3 is the trailing quote + anything after.
var rxSchemaPointer = regexp.MustCompile(
	`(?m)^(\s*#\s*yaml-language-server:\s*\$schema\s*=\s*["']?)` +
		`(https?://[^\s"'#]*taskfile\.dev[^\s"'#]*)` +
		`(["']?.*)$`,
)

// riteSchemaURL is the hosted schema for Ritefiles; see clintmod/rite #73.
const riteSchemaURL = "https://clintmod.github.io/rite/schema/v3.json"

// rewriteSchemaPointer swaps any yaml-language-server `$schema=` URL that
// points at taskfile.dev to rite's hosted schema, preserving indentation,
// surrounding whitespace, quoting, and any trailing comment. Users who
// pointed at a non-taskfile.dev schema chose it deliberately and are left
// alone. Files without the directive get nothing injected.
func rewriteSchemaPointer(s string) string {
	return rxSchemaPointer.ReplaceAllString(s, "${1}"+riteSchemaURL+"$3")
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

// modernizeTemplates walks every `{{ … }}` expression in s. Expressions
// that are simple variable references (or `default`-pipe variants) are
// rewritten to the equivalent rite-native `${VAR}` / `${VAR:-fallback}`
// form — both resolve through `internal/templater.ExpandShell` and the
// mvdan/sh cmd interpreter so the semantics round-trip for exported
// vars (the common case). Expressions that use Go-template control flow
// (`if` / `range`), function calls (`printf`, `index`, sprig helpers),
// or any multi-step pipe beyond the `default` shape are left untouched
// and reported via a TEMPLATE-KEPT warning so the user can rewrite them
// by hand.
//
// Idempotent — a Ritefile that already uses `${VAR}` has no `{{}}`
// matches for this pass to touch.
//
// Caveat: `{{.VAR | default "x"}}` rewrites to `${VAR:-x}`, which
// resolves via the shell interpreter at cmd-exec time. For rite vars
// marked `export: false` that default is consulted *even when the
// rite-side var is set*, because the cmd shell only sees exported vars.
// That's a semantic difference from the Go-template form, which sees
// every rite var regardless of export. The shell-native form is still
// the idiomatic rewrite — users who hit this can opt out with
// `--keep-go-templates` or add `export: true` if the default was
// previously silently unreachable.
func modernizeTemplates(s, srcPath string, warn io.Writer) string {
	matches := rxTemplateExpr.FindAllStringIndex(s, -1)
	if len(matches) == 0 {
		return s
	}
	var out strings.Builder
	out.Grow(len(s))
	prev := 0
	for _, m := range matches {
		out.WriteString(s[prev:m[0]])
		expr := s[m[0]:m[1]]
		if repl, ok := modernizeTemplateExpr(expr); ok {
			out.WriteString(repl)
		} else {
			out.WriteString(expr)
			line := 1 + strings.Count(s[:m[0]], "\n")
			fmt.Fprintf(warn,
				"rite migrate: TEMPLATE-KEPT %s:%d: kept Go-template syntax %q — no equivalent ${VAR} form; review manually.\n",
				srcPath, line, expr)
		}
		prev = m[1]
	}
	out.WriteString(s[prev:])
	return out.String()
}

// modernizeTemplateExpr maps a full `{{ … }}` match to the equivalent
// shell-preprocessor form, or returns false if the expression isn't a
// shape we know how to translate. Shapes handled:
//
//	{{ .VAR }}                      -> ${VAR}
//	{{ .VAR | default "x" }}        -> ${VAR:-x}
//	{{ .VAR | default 'x' }}        -> ${VAR:-x}
//	{{ .VAR | default .OTHER }}     -> ${VAR:-${OTHER}}
//	{{ default "x" .VAR }}          -> ${VAR:-x}   (alternate ordering)
//	{{ default .OTHER .VAR }}       -> ${VAR:-${OTHER}}
//
// Leading/trailing `-` whitespace-trim markers (`{{- … -}}`) are
// tolerated and discarded — rite's shell-preprocessor form has no
// equivalent trim marker, and the trim behavior is effectively a no-op
// once the rewrite is in place (no surrounding whitespace is produced
// by `${VAR}` expansion itself).
func modernizeTemplateExpr(expr string) (string, bool) {
	inner := strings.TrimSpace(expr[2 : len(expr)-2])
	inner = strings.TrimPrefix(inner, "-")
	inner = strings.TrimSuffix(inner, "-")
	inner = strings.TrimSpace(inner)

	if m := rxBareVar.FindStringSubmatch(inner); m != nil {
		return "${" + m[1] + "}", true
	}
	if m := rxPipeDefault.FindStringSubmatch(inner); m != nil {
		def, ok := modernizeDefaultArg(m[2])
		if !ok {
			return "", false
		}
		return "${" + m[1] + ":-" + def + "}", true
	}
	if m := rxPrefixDefault.FindStringSubmatch(inner); m != nil {
		def, ok := modernizeDefaultArg(m[1])
		if !ok {
			return "", false
		}
		return "${" + m[2] + ":-" + def + "}", true
	}
	return "", false
}

// modernizeDefaultArg translates a Go-template `default` argument to its
// shell-preprocessor form: a quoted string becomes its unquoted payload,
// a dotted identifier becomes a `${…}` reference. Returns false on any
// shape we don't translate (bare numbers, function calls, nested pipes).
func modernizeDefaultArg(arg string) (string, bool) {
	arg = strings.TrimSpace(arg)
	if len(arg) >= 2 {
		first, last := arg[0], arg[len(arg)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			// Reject embedded quotes of the same kind — they break the
			// naive unquote and almost never appear in real Taskfiles.
			inner := arg[1 : len(arg)-1]
			if strings.ContainsRune(inner, rune(first)) {
				return "", false
			}
			return inner, true
		}
	}
	if m := rxDottedIdent.FindStringSubmatch(arg); m != nil {
		return "${" + m[1] + "}", true
	}
	return "", false
}

var (
	rxBareVar       = regexp.MustCompile(`^\.([A-Za-z_][A-Za-z0-9_]*)$`)
	rxDottedIdent   = regexp.MustCompile(`^\.([A-Za-z_][A-Za-z0-9_]*)$`)
	rxPipeDefault   = regexp.MustCompile(`^\.([A-Za-z_][A-Za-z0-9_]*)\s*\|\s*default\s+(.+?)$`)
	rxPrefixDefault = regexp.MustCompile(`^default\s+(.+?)\s+\.([A-Za-z_][A-Za-z0-9_]*)$`)
)

// rewriteSelfRefCmds rewrites every `task` CallExpr head in every shell-cmd
// scalar to `rite`. Issue #128: a migrated Ritefile that still shells out
// to the `task` binary either silently runs the old go-task (against a
// Ritefile it can't find) or fails outright once the user has uninstalled
// it. We close the loop by rewriting self-referential CLI invocations at
// migrate time.
//
// Approach: walk the YAML to locate every shell-cmd scalar (cmd:, defer:,
// sh:, and bare list items under cmds:), parse each value with mvdan/sh,
// and rewrite *only* CallExpr nodes whose first Word is the literal
// `task`. This is what catches the easy `task lint` cases AND the
// adversarial ones — `cd sub && task build`, `$(task --version)`,
// quoted strings, heredocs — that a regex would either miss or
// false-positive on.
//
// What this does NOT rewrite:
//
//   - `echo task is cool` — `task` is a Word in the args, not the head of
//     a CallExpr. Word-position occurrences (filenames, prose, flag
//     values) are unambiguous to the shell parser.
//   - `./task` / `bin/task` / `mytask` — the head Lit's value is `./task`
//     / `mytask`, not the bare token `task`.
//   - The YAML key `task:` (a structural keyword for the call-another-task
//     cmd shape) — we operate on string values, not keys.
//   - Cmds that fail to shell-parse — left untouched and reported with a
//     SELFREF-CMD warning so the user can hand-review the line.
//
// Substitution strategy: for each rewrite-candidate scalar we know its
// node line number from yaml.v3. We rewrite the line in place with a
// single strings.Replace of the original value with the rewritten value
// (line-scoped so we never collide with a same-text scalar elsewhere).
// Block scalars (literal `|` / folded `>`) span multiple lines and don't
// fit single-line substitution; they're skipped with a warning rather
// than rewritten naively.
func rewriteSelfRefCmds(s, srcPath string, warn io.Writer) string {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(s), &doc); err != nil {
		// The file already round-tripped through yaml.v3 in
		// emitMigratedFile; if we can't reparse here something's wrong
		// with the rewritten output. Better to no-op than to corrupt.
		return s
	}
	scalars := collectShellCmdScalars(&doc)
	if len(scalars) == 0 {
		return s
	}

	lines := strings.Split(s, "\n")
	for _, sc := range scalars {
		// yaml.Node.Line is 1-indexed; line 0 means the node was
		// constructed without source position info (shouldn't happen
		// for a freshly-unmarshaled doc, but be defensive).
		if sc.line < 1 || sc.line > len(lines) {
			continue
		}
		// Block scalars (literal `|` / folded `>`) put the value on
		// the lines AFTER the indicator. Single-line substring replace
		// won't find the value on sc.line; rather than try to
		// reconstruct the indented body, skip and warn so the user
		// can handle it manually. SglQuoted/DblQuoted/plain styles
		// keep the value on a single line.
		if sc.style == yaml.LiteralStyle || sc.style == yaml.FoldedStyle {
			rewritten, _, ok := rewriteShellCmd(sc.value)
			if !ok || rewritten == sc.value {
				continue
			}
			fmt.Fprintf(warn,
				"rite migrate: SELFREF-CMD %s:%d: skipped block scalar — rewrite needed but cmd spans multiple source lines; edit by hand.\n",
				srcPath, sc.line)
			continue
		}
		rewritten, parseOK, ok := rewriteShellCmd(sc.value)
		if !parseOK {
			fmt.Fprintf(warn,
				"rite migrate: SELFREF-CMD %s:%d: skipped — shell parse error on cmd %q; review manually.\n",
				srcPath, sc.line, sc.value)
			continue
		}
		if !ok || rewritten == sc.value {
			continue
		}
		idx := sc.line - 1
		// Replace exactly once: a single cmd value appears once on its
		// source line (YAML idiom). Multi-occurrence pathologies (the
		// same string twice on one line) would over-rewrite, but
		// they're not realistic in cmd scalars.
		if !strings.Contains(lines[idx], sc.value) {
			// The value didn't survive verbatim on the source line
			// (escaped quoting, line continuation, etc.). Skip with
			// a warning rather than guess.
			fmt.Fprintf(warn,
				"rite migrate: SELFREF-CMD %s:%d: skipped — could not locate cmd value %q on its source line; review manually.\n",
				srcPath, sc.line, sc.value)
			continue
		}
		lines[idx] = strings.Replace(lines[idx], sc.value, rewritten, 1)
		fmt.Fprintf(warn,
			"rite migrate: SELFREF-CMD %s:%d: rewrote `task` CLI invocation to `rite`.\n",
			srcPath, sc.line)
	}
	return strings.Join(lines, "\n")
}

// shellCmdScalar pairs a YAML scalar's source line with its decoded value
// and quoting style, so the line-level substitution in
// rewriteSelfRefCmds knows where to apply the rewrite and which scalars
// need to be skipped because they span multiple source lines.
type shellCmdScalar struct {
	line  int
	value string
	style yaml.Style
}

// collectShellCmdScalars walks the YAML document and returns every scalar
// node positioned in a shell-cmd-bearing slot. The recognized slots are:
//
//   - bare scalar items in a sequence whose key is `cmds`
//   - the value scalar of a `cmd:` mapping key
//   - the value scalar of a `defer:` mapping key (the deferred-cmd shape)
//   - the value scalar of a `sh:` mapping key (preconditions + dynamic
//     vars — both run their value through the shell)
//
// Anything else (var values, env values, descriptions, prompts, etc.) is
// skipped — those positions don't go through the cmd shell so a literal
// `task` token in them isn't a CLI invocation.
func collectShellCmdScalars(root *yaml.Node) []shellCmdScalar {
	var out []shellCmdScalar
	walkYAML(root, "", func(parentKey string, n *yaml.Node) {
		if n.Kind != yaml.ScalarNode {
			return
		}
		switch parentKey {
		case "cmd", "defer", "sh", "@cmds-item":
			out = append(out, shellCmdScalar{
				line:  n.Line,
				value: n.Value,
				style: n.Style,
			})
		}
	})
	return out
}

// walkYAML recursively visits each node, calling visit(parentKey, node).
// parentKey is the YAML mapping key whose value contains node, or
// `@cmds-item` for the special case of a scalar item directly inside a
// `cmds:` sequence. Other sequence items get an empty parentKey.
func walkYAML(n *yaml.Node, parentKey string, visit func(parentKey string, n *yaml.Node)) {
	if n == nil {
		return
	}
	visit(parentKey, n)
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			walkYAML(c, "", visit)
		}
	case yaml.MappingNode:
		// Content alternates [key, value, key, value, ...].
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i]
			val := n.Content[i+1]
			keyName := ""
			if key.Kind == yaml.ScalarNode {
				keyName = key.Value
			}
			if val.Kind == yaml.SequenceNode && keyName == "cmds" {
				// Scalar items directly under cmds: are bare cmds;
				// mapping items are full cmd structs (cmd:/defer:/etc.)
				// and get visited via the regular walk below so their
				// inner cmd:/defer: keys are picked up.
				for _, item := range val.Content {
					if item.Kind == yaml.ScalarNode {
						visit("@cmds-item", item)
					} else {
						walkYAML(item, "", visit)
					}
				}
				continue
			}
			walkYAML(val, keyName, visit)
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			walkYAML(c, "", visit)
		}
	}
}

// rewriteShellCmd parses cmd as a shell snippet, finds every CallExpr
// whose head Word is the literal `task`, and substitutes `rite` for the
// `task` token at the exact byte offset reported by mvdan/sh.
//
// The bool results are:
//
//   - parseOK: the shell parser accepted the cmd. If false, the caller
//     should leave the cmd untouched and emit a warning.
//   - changed: at least one head literal was rewritten.
//
// Why offset-substitution instead of printer round-trip: mvdan/sh's
// `syntax.NewPrinter().Print` is canonical, not preserving — it turns
// `task a; task b` into `task a\ntask b` and backticks into `$(...)`.
// That's fine for the parsed semantics but ruins the line-scoped
// substitution `rewriteSelfRefCmds` does back into the YAML source
// (a multi-line cmd value can't be re-injected into a single source
// line). Using `lit.ValuePos.Offset()` to splice the rewrite into the
// original text preserves all whitespace, separators, and quoting
// around the rewritten word — which is what we need.
func rewriteShellCmd(cmd string) (rewritten string, parseOK bool, changed bool) {
	parser := syntax.NewParser()
	file, err := parser.Parse(strings.NewReader(cmd), "")
	if err != nil {
		return cmd, false, false
	}
	type hit struct{ start, end int }
	var hits []hit
	syntax.Walk(file, func(node syntax.Node) bool {
		ce, ok := node.(*syntax.CallExpr)
		if !ok || len(ce.Args) == 0 {
			return true
		}
		head := ce.Args[0]
		if len(head.Parts) != 1 {
			return true
		}
		lit, ok := head.Parts[0].(*syntax.Lit)
		if !ok {
			return true
		}
		if lit.Value != "task" {
			return true
		}
		hits = append(hits, hit{
			start: int(lit.ValuePos.Offset()),
			end:   int(lit.ValueEnd.Offset()),
		})
		return true
	})
	if len(hits) == 0 {
		return cmd, true, false
	}
	// Splice from the back so earlier offsets stay valid.
	var buf bytes.Buffer
	prev := 0
	for _, h := range hits {
		if h.start < prev || h.end > len(cmd) || h.end < h.start {
			// Defensive: bail and leave the cmd untouched if mvdan/sh
			// reported an out-of-range offset (shouldn't happen).
			return cmd, true, false
		}
		buf.WriteString(cmd[prev:h.start])
		buf.WriteString("rite")
		prev = h.end
	}
	buf.WriteString(cmd[prev:])
	return buf.String(), true, true
}

// RewriteShellCmdForTest exports rewriteShellCmd so unit tests in
// migrate_test.go can drive the rewriter directly without spinning up a
// full migrate cycle. Returns the rewritten cmd plus the same parseOK /
// changed flags rewriteShellCmd uses internally.
func RewriteShellCmdForTest(cmd string) (string, bool, bool) {
	return rewriteShellCmd(cmd)
}

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
