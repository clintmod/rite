# Feature-Map Audit: Rite Post-Fork Feature Implementation

This audit verifies the presence and behavior of 19 key documentation features across the `rite` codebase, checking for existence, divergence from upstream go-task behavior, implementation locations, and test coverage. **rite** is a hard fork of go-task/task with first-in-wins variable precedence (shell env > CLI > Ritefile env > task env), which changes variable and environment handling semantics compared to upstream. Each feature row includes status, source files, test/fixture reference, divergence notes (if any), and documentation emphasis guidance.

---

## Feature Audit Table

| REQ | Feature | Status | Source Files | Test/Fixture | Divergence | Docs Emphasis |
|-----|---------|--------|--------------|--------------|-----------|---|
| **DOCS-01** | **Internal tasks** (`internal: true`) | **Exists** | `/taskfile/ast/task.go:37` (field), `/completion.go:48` (filter), `/task.go:63` (check) | `/testdata/internal_task/Ritefile.yml`, `/executor_test.go` (no direct test but feature used) | None — behaves identically to go-task | Excluded from `--list` and direct CLI invocation; callable only via `task:` command or dependency |
| **DOCS-02** | **Task working directory** (`dir:` field) | **Exists** | `/taskfile/ast/task.go:29`, `/compiler.go:160` (template resolution), `/variables.go` (dir scoping) | `/testdata/dir/Ritefile.yml`, `/testdata/dir/dynamic_var/` | None — relative path resolution unchanged | `dir:` paths are relative to Ritefile location; resolved after Tiers 1–4 of variable precedence so templates like `{{.DIR_VAR}}` can reference entrypoint vars |
| **DOCS-03** | **Platform-specific tasks** (`platforms:` field) | **Exists** | `/taskfile/ast/platforms.go` (parsing), `/taskfile/ast/task.go:42` | `/testdata/platforms/Ritefile.yml` | None — OS/arch gating unchanged | Specify `platforms: [windows]` or `platforms: [linux/amd64]` to skip task on non-matching OS/arch; platform-gated tasks skip silently with no error |
| **DOCS-04** | **Calling another task** (`task:` in `cmds:`) | **Exists** | `/taskfile/ast/cmd.go:93–101` (unmarshaler), `/task.go` (executor), `/completion.go:56` | Multiple testdata files (`deferred/`, `alias/`), `/executor_test.go:TestAlias`, `/executor_test.go:TestForDeps` | None — task calling unchanged | `vars:` forwarding under first-in-wins: explicit `vars:` in task call are Tier 2 (CLI precedence) so they override entrypoint env/vars but lose to shell env |
| **DOCS-05** | **CLI args forwarding** (`.CLI_ARGS` / `CLI_ARGS` special var) | **Exists** | `/cmd/rite/task.go` (line w/ `specialVars.Set`), `/taskfile/ast/var.go` | `/executor_test.go:TestSpecialVars` (special vars tested) | None — passthrough unchanged | `CLI_ARGS` and `CLI_ARGS_LIST` populated from args after `--`; not exported by default (`export: false`); available for template expansion in tasks |
| **DOCS-06** | **Wildcard task names** (`*` in task names) | **Exists** | `/taskfile/ast/task.go:79–103` (WildcardMatch), `/variables.go` (MATCH var handling) | `/testdata/wildcards/Ritefile.yml`, matches in fixture output | None — wildcard matching unchanged | `{{index .MATCH 0}}` or `{{.MATCH}}` expands captured groups; empty MATCH when invoked with exact task name (not `*` expanded) |
| **DOCS-07** | **Defer cleanup** (`defer:` inside `cmds:`) | **Exists** | `/taskfile/ast/defer.go` (Defer struct), `/taskfile/ast/cmd.go:72–90` (unmarshaler), `/task.go:450–500` (deferral queue) | `/testdata/deferred/Ritefile.yml` (comprehensive), `/executor_test.go` (no direct test but deferral tested indirectly) | None — deferral semantics unchanged | Deferred commands/tasks run after main cmds on both success and failure; allow cleanup; use `silent: true` to hide cleanup output |
| **DOCS-08** | **Task aliases** (`aliases:` field) | **Exists** | `/taskfile/ast/task.go:24` (field), `/taskfile/ast/task.go:79–103` (WildcardMatch includes aliases) | `/testdata/alias/Ritefile.yml`, `/executor_test.go:TestAlias` | None — alias behavior unchanged | Alternate invocation names; included in summary/list output; matched same way as primary task name |
| **DOCS-09** | **Label override** (`label:` field) | **Exists** | `/taskfile/ast/task.go:19,55–63` (Name() method prioritizes label), `/variables.go:48` (templated in list) | `/testdata/label_list/Ritefile.yml`, `/testdata/label_var/Ritefile.yml`, `/executor_test.go:TestLabel` | None — label unchanged | Custom display name in output and list; overrides task name for UI; templated, so `label: "Task {{.ENV}}"` is valid |
| **DOCS-10** | **Warning prompts** (`prompt:` field) | **Exists** | `/taskfile/ast/prompt.go` (Prompt type), `/taskfile/ast/task.go:21` (field), `/task.go` (prompt execution pre-run) | `/testdata/prompt/Ritefile.yml` (multi-prompt syntax), `/executor_test.go:TestPromptInSummary`, `/executor_test.go:TestPromptAssumeYes` | None — prompt unchanged | Single string or array of strings; requires user confirmation before task runs; skipped if `--yes` flag set; templated |
| **DOCS-11** | **Silent/dry-run/ignore-errors** (`silent:`, `--dry`, `ignore_error:`) | **Exists** | `/taskfile/ast/task.go:35` (Silent bool), `/taskfile/ast/cmd.go:16` (cmd-level), `/internal/flags/flags.go:62,67,143` (CLI flags) | `/testdata/env/Ritefile.yml` (silent:true), `/testdata/deferred/` (silent in defer), flags tested in integration | None — behavior unchanged | `silent: true` suppresses echo of task output; `--dry` prints task order w/o execution; `ignore_error: true` allows task to continue on failure (per-cmd or per-task) |
| **DOCS-12** | **Set and shopt** (`set:` / `shopt:` fields) | **Exists** | `/taskfile/ast/task.go:30,31` (fields), `/taskfile/ast/cmd.go:17,18` (cmd-level), `/task.go` (shell invocation) | `/testdata/shopts/task_level/Ritefile.yml`, `/testdata/shopts/command_level/Ritefile.yml` (shopt/set examples) | None — shell options unchanged | `set: [pipefail, ...]` runs `set -o pipefail ...` before cmd; `shopt: [globstar, ...]` runs `shopt -s globstar ...`; at task or cmd scope |
| **DOCS-13** | **Watch mode** (`--watch` flag, `sources:`) | **Exists** | `/internal/flags/flags.go:62,136,154` (Watch, Interval flags), `/task.go` (file watching), `/testdata/watch/Ritefile.yaml` | `/testdata/watch/Ritefile.yaml`, `/testdata/run_when_changed/` (sources-based re-run), `/executor_test.go` (integration tested) | None — watch unchanged | `rite --watch taskname` watches `sources:` for changes and re-runs task on file modification; configurable with `--interval` |
| **DOCS-14** | **Interactive CLI apps** (`interactive: true` for TUIs/REPLs) | **Exists** | `/taskfile/ast/task.go:36` (field), `/internal/flags/flags.go:90,141` (flag & config), `/task.go` (TTY allocation) | `/testdata/interactive_vars/Ritefile.yml`, `/testdata/interactive_vars/.taskrc.yml` (config example) | None — interactivity unchanged | `interactive: true` allocates PTY for task so TUI/REPL sees stdin/terminal; enables interactive prompts in requires; can be set globally in `.taskrc.yml` |
| **DOCS-15** | **Help / list / list-all / summary** (`--list`, `--list-all`, `--summary`, `--list-json`) | **Exists** | `/internal/flags/flags.go:52–55,128–130,144` (flag definitions), `/task.go:GetTaskList`, `/completion.go` (filtering) | `/testdata/list_desc_interpolation/`, `/testdata/label_list/`, `/testdata/json_list_format/`, `/executor_test.go:TestLabel`, `/executor_test.go:TestSummaryWithVarsAndRequires` | None — list behavior unchanged | `--list` shows tasks with descriptions; `--list-all` includes tasks without descriptions; `--summary` shows task + desc + alias + requires; `--json` formats as JSON with nesting |
| **DOCS-16** | **CI integration** (GitHub Actions annotations, color detection) | **Exists** | `/cmd/rite/task.go:48–57` (emitCIErrorAnnotation), `/internal/flags/flags.go:182–197` (color auto-detect) | Integration via CI env; color tested implicitly via flag logic | None — CI unchanged | Detects `GITHUB_ACTIONS` env and emits `::error` annotations; respects `NO_COLOR` env; auto-enables colors in CI if `FORCE_COLOR` set; `-c` / `--color` overrides |
| **DOCS-17** | **Short task syntax** (`mytask: echo hi` shorthand) | **Exists** | `/taskfile/ast/task.go:105–124` (UnmarshalYAML scalar & seq cases) | Multiple testdata files use shorthand; simple string cmds throughout | None — syntax unchanged | Single string `task: cmd` or array `task: [cmd1, cmd2]` automatically becomes a task with that command in `cmds:`; object form allows full options |
| **DOCS-18** | **Env block precedence** (`env:` at Ritefile & task scope) | **Exists** | `/taskfile/ast/taskfile.go:28` (Ritefile.Env), `/taskfile/ast/task.go:33` (task.Env), `/variables.go:138–175` (tiers 3–6), `/compiler.go:44–62` (precedence docs) | `/testdata/env/Ritefile.yml` (task-level env override test), `/executor_test.go:TestEnv` | **Differs**: First-in-wins means shell env > CLI vars > Ritefile env > task env; task-level `env:` are *defaults* not overrides. Task env does not override Ritefile env or shell vars — highest-priority tier wins | **Critical**: Emphasize first-in-wins on env block documentation. Shell environment and CLI vars override Ritefile-level env. Task-scope env vars are fallback defaults only. Task dotenv (`dotenv:`) merged before task env in the same tier. Include "Differences from go-task" callout about variable precedence inversion. |
| **DOCS-19** | **Sidebar config** (VitePress structure) | **Exists** | `/website/.vitepress/config.ts:29–61` (sidebar array) | N/A (static config file) | N/A — rite-specific structure | **Current structure**: "Start here" (2 items), "Features" (6 items: deps, sources-and-generates, includes, run-modes, preconditions, for-loops), "Reference" (4 items: precedence, syntax, schema, cli), "Coming from go-task" (1 item: migration). Extensible for 14 new user-guide pages via additional `text/link` objects in existing sections or new sections. |

---

## Findings Summary

### (A) Features Diverging from go-task (Require "Differences" Callouts)

**DOCS-18 (Env block precedence)** is the **primary divergence**. Under first-in-wins semantics:
- Shell environment variables take **absolute priority** over all Ritefile/task declarations
- CLI-provided `FOO=bar rite task` beats Ritefile env
- Task-scope `env:` becomes a fallback tier, not an override
- This is opposite to go-task's behavior where task env overrides entrypoint env

This requires a prominent "Differences from go-task" callout on every env/variable-related page, with a side-by-side comparison table showing the precedence difference.

### (B) Missing Features (Not in rite post-fork)

**None of the 19 audit items are missing.** All documented features exist and are functional. No features need to be dropped from the milestone.

### (C) Surprising Discoveries / Key Notes

1. **Variable cache design** (DOCS-18): rite's compiler uses a two-cache system (SPEC §Dynamic Variables) where special vars (`RITEFILE`, `TASK`, etc.) are seeded into the templater cache but not merged into result vars until Tier 8, preserving first-in-wins semantics. This is non-obvious and worth documenting.

2. **Deferred scope** (DOCS-07): Deferred commands and tasks inherit variables from the *time they run* (end of task), not the time they were declared. This is correct but easy to misunderstand.

3. **Label overrides task name in output**: Labels are templated and can include variables, making them dynamic. Worth emphasizing in DOCS-09.

4. **Interactive requires** (DOCS-14): The `interactive: true` flag is key to making `requires:` prompts work in TTY-allocated shells. Worth calling out in any requires documentation.

5. **Platform-gated deps don't error on skip**: If a dependency is skipped due to platform mismatch, there's no error — tasks silently skip. This is go-task behavior preserved here.

6. **VitePress sidebar is ready to extend**: Current config has 13 items across 4 sections. Adding 14 new user-guide pages is straightforward; consider grouping by feature area (e.g., "Variables & Environment", "Task Execution", "Advanced Features").

---

## Documentation Checklist

- [ ] DOCS-01–09: No upstream divergence; document feature as-is
- [ ] DOCS-10–17: No divergence; document feature as-is
- [ ] **DOCS-18: CRITICAL — Add "Differences from go-task" callout with precedence table**
- [ ] DOCS-19: Extend sidebar with new sections as pages are written
- [ ] All feature pages: Link to `/precedence` for variable precedence details
- [ ] All env/var pages: Use `${VAR}` preprocessor style in examples (not `{{.VAR}}`)

