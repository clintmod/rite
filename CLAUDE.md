# rite — Claude Operating Manual

Load this before doing any work on `rite`. It is committed to git and is the authoritative project brief for both human and AI contributors.

---

## Mission

`rite` is a task runner with Unix-native variable precedence. It exists because `go-task`'s variable model inverts Unix semantics (task-level `vars:` override CLI and shell env) and the upstream project has publicly committed to keeping that inversion in their planned redesign ([go-task/task#2035](https://github.com/go-task/task/issues/2035)).

`rite` is a **hard fork** of `go-task/task` — no compatibility shim, no intention to merge upstream, one-way migration only.

See [`SPEC.md`](./SPEC.md) for the design contract. Any disagreement between this file and `SPEC.md` is resolved in favor of the SPEC.

---

## Relationship to upstream

- `git remote -v` shows two remotes: `origin` (this repo) and `upstream` (go-task/task).
- Cherry-picks from `upstream/main` are allowed for security, CI, platform support, and other non-variable-system work.
- Never merge upstream branches wholesale. Never open PRs back to upstream.
- Anything touching `compiler.go`, `variables.go`, `internal/templater/`, `internal/env/`, `taskfile/`, or `taskfile/ast/vars*.go` is ours — do not pull upstream changes in those files.

---

## Development workflow

**Build and install:**
```bash
rite install   # go install ./cmd/rite + PATH-aware symlink fallback
```
The `install:` task runs `go install` and, if `$(go env GOBIN || go env GOPATH)/bin` isn't on `$PATH`, walks XDG-first candidates (`~/.local/bin`, `~/bin`, `/usr/local/bin`) and symlinks the binary into the first one already on `$PATH`. `rite uninstall` undoes both the `go install` and any candidate symlinks (idempotent).

**Smoke test:**
```bash
cd /tmp && mkdir -p rite-smoketest && cd rite-smoketest
cat > Ritefile <<'EOF'
version: '3'
tasks:
  default:
    cmds:
      - echo "{{.RITE_VERSION}} at {{.RITEFILE}}"
EOF
rite
```

**Run the project's own tasks:**
```bash
rite lint
rite test
```

**Enable the shared git hooks** (one-time per clone — runs `golangci-lint` on pre-push so CI doesn't eat gci failures):
```bash
rite hooks
```

**File discovery:** `Ritefile`, `Ritefile.yml`, `Ritefile.yaml`, `Ritefile.dist.yml`, `Ritefile.dist.yaml` (plus lowercase variants). No Taskfile recognition.

**Env var prefix:** `RITE_*` for rite-internal config. `RITE_X_*` for experiment flags (e.g. `RITE_X_GENTLE_FORCE=1`).

**Regenerating golden fixtures:** always pass both env vars or the snapshots bake in the local absolute path and CI explodes on cross-platform runners:
```bash
GOLDIE_UPDATE=true GOLDIE_TEMPLATE=true go test ./... -run <TestName>
```
The Ritefile's `generate:fixtures` target already sets both.

---

## Phase status — all shipped

- **Phase 0 (done):** Repo identity — SPEC, LICENSE, NOTICE, README.
- **Phase 1 (done `bebe02bc`):** Module path, binary, file discovery, env var prefix, special var rename.
- **Phase 1.5 (done `d096a229`, `4600626f`):** Log prefix + error strings + `rite --init`.
- **Phase 2 (done `8d7ebd7f`, `ffb9838e`):** First-in-wins `getVariables()`, per-resolution dynamic cache.
- **Phase 3 (done `6183bc96` through `01c62390`):** Fixture rebrand + rewrite, include-site var precedence fix (`Taskfile.Merge` no longer flattens).
- **Phase 4 (done `da018dc6`, `419f2f96`, `75930421`):** `${VAR}` preprocessor, `export: false`, vars/env unified, shell env always wins.
- **Phase 5 (done `f1d1d121` through `82dbd415`):** `rite --migrate` flag, docs site at clintmod.github.io/rite, Homebrew tap at clintmod/homebrew-tap, v0.1.0 tagged and published.

---

## Shipped infrastructure

- **Releases:** `v0.1.0` published, archives for darwin/linux/windows/freebsd × amd64/arm64/arm/386/riscv64 + deb/rpm/apk. `goreleaser` runs on every `v*` tag. Config at `.goreleaser.yml` uses free goreleaser; `release.draft: false` so tags auto-publish.
- **Homebrew tap:** `clintmod/homebrew-tap/Formula/rite.rb`. Pushed by goreleaser using the `HOMEBREW_TAP_TOKEN` secret (fine-grained PAT scoped to the tap only). Main repo release uses the default `GITHUB_TOKEN`. Users install: `brew tap clintmod/tap && brew install rite`.
- **Docs site:** `website/` is a pure VitePress project; `.github/workflows/pages.yml` builds on every push that touches `website/**` and publishes to `clintmod.github.io/rite/` via GitHub Pages (Source: GitHub Actions — enabled via `gh api POST /repos/clintmod/rite/pages`). Custom domain can be added later by dropping `DOCS_BASE=/rite/` from `pages.yml`.
- **CI:** Test (Go 1.26 × ubuntu+macos+windows), Lint (golangci-lint v2.11.4, config at `.golangci.yml`), Docs, goreleaser. All green on main at session end.
- **Secrets on `clintmod/rite`:** `GH_PAT` (fine-grained, scoped to `clintmod/homebrew-tap`, exposed to workflow as `HOMEBREW_TAP_TOKEN`).

---

## Known sharp edges

- **Core library lives at `internal/task/`, package name `task`.** The root of the module used to hold the library; it was moved under `internal/task/` to tidy the tree (rite is a CLI, not an import target). Most external callers (`cmd/rite/`, `args/`, `internal/flags/`) still use an explicit `task "github.com/clintmod/rite/internal/task"` alias — gci groups it under the local-module section. The alias is now redundant (path basename matches package name) but kept for consistency; goimports/golangci-lint won't flag either form.
- **Embedded assets moved with the package.** `internal/task/completion/{bash,fish,ps,zsh}/...` and `internal/task/templates/default.yml` used to live at the repo root; they have to sit inside the package that `//go:embed`s them. Same for `internal/task/testdata/` (Go tests resolve `testdata/` relative to the test binary's package dir).
- **VitePress + Shiki + Vue bug (broader than originally documented):** the Vue SFC compiler tokenizes every `{{…}}` it finds *anywhere* in rendered HTML, including inside `<pre><code>` from fenced blocks. Symptoms: `src/foo.md (L:C): Error parsing JavaScript expression`, with positions that don't match your source. Triggers:
  - **Multi-expression-per-line:** `cmd: GOOS={{.GOOS}} GOARCH={{.GOARCH}}` — sometimes works, sometimes not, depends on whether Vue can parse the JS-shaped content between markers.
  - **Go-template helpers with positional args:** `{{ index .MATCH 0 }}` — the literal `0` after `.MATCH` is invalid JS, hard error.
  - **Pipe inside `{{}}`:** `{{splitArgs .CLI_ARGS | len}}` — Vue interprets `|` as filter syntax.
  - **`if/else/end` blocks:** `{{if .CI}}foo{{else}}bar{{end}}` — every directive keyword is invalid JS.
  - **Building from project root:** `npx vitepress build` from `/Users/clint/code/github/clintmod/rite/` (not `website/`) walks the root `CHANGELOG.md` and `README.md` and trips on their `{{.VAR}}` examples. **Always build from `website/`.**
  - Workarounds in order of effectiveness: (1) prefer `${VAR}` shell-preprocessor form everywhere — SPEC-preferred and Vue-invisible. (2) For inline prose, `<span v-pre>``{{.VAR}}``</span>` works. (3) For fenced blocks containing unavoidable Go-template syntax, the `<div v-pre>` / `<span v-pre>` wrapper does **not** work — Markdown wraps the fence in `<pre><code>` first. The pragmatic move is to describe the syntax in prose with backticks (which Vue tolerates) and put a working example in `testdata/` referenced by URL.
- **Older mise + `ubi:` backend strips `v` prefixes.** Users on mise < 2026.4 hitting `"ubi:clintmod/rite" = "v0.1.0"` get a 404. Docs steer them to the `go:` backend fallback. When bumping mise in CI, use 2026.4.x+.
- **`lint-jsonschema` job is gone until we publish a schema.** The upstream workflow validated `website/src/public/schema.json` (Taskfile schema). We deleted the file with the site rebuild; re-add the job when we host our own at `clintmod.github.io/rite/schema.json`.

---

## Conventions

- **Commits:** Prefix with the phase when relevant (`rebrand:`, `precedence:`, `migrate:`). No emoji. Explain the *why*, not just the *what*.
- **No partial rebrands.** Any rename/restructure should land in a single atomic commit across the codebase so the tree is always buildable.
- **SPEC before code.** Behavioral changes that touch variable semantics require a SPEC update first or in the same commit.
- **Don't silently pull from upstream.** Every cherry-pick needs an explicit `git cherry-pick -x` (records the source) and a note in the commit message about why we want it.
- **No subcommands.** See [`SPEC.md`](./SPEC.md) §Out of Scope. `rite <positional>` is always a task name; non-task invocations are flags. This principle cost us a same-day revert in 1.0 (#83); do not re-propose a subcommand without proposing a SPEC change first.
