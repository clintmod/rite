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
go install ./cmd/rite
# Symlinks at ~/bin/rite → ~/go/bin/rite are set up on the dev machine.
```

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

**File discovery:** `Ritefile`, `Ritefile.yml`, `Ritefile.yaml`, `Ritefile.dist.yml`, `Ritefile.dist.yaml` (plus lowercase variants). No Taskfile recognition.

**Env var prefix:** `RITE_*` for rite-internal config. `RITE_X_*` for experiment flags (e.g. `RITE_X_ENV_PRECEDENCE=1`).

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
- **Phase 5 (done `f1d1d121` through `82dbd415`):** `rite migrate` subcommand, docs site at clintmod.github.io/rite, Homebrew tap at clintmod/homebrew-tap, v0.1.0 tagged and published.

---

## Shipped infrastructure

- **Releases:** `v0.1.0` published, archives for darwin/linux/windows/freebsd × amd64/arm64/arm/386/riscv64 + deb/rpm/apk. `goreleaser` runs on every `v*` tag. Config at `.goreleaser.yml` uses free goreleaser; `release.draft: false` so tags auto-publish.
- **Homebrew tap:** `clintmod/homebrew-tap/Formula/rite.rb`. Pushed by goreleaser using the `HOMEBREW_TAP_TOKEN` secret (fine-grained PAT scoped to the tap only). Main repo release uses the default `GITHUB_TOKEN`. Users install: `brew tap clintmod/tap && brew install rite`.
- **Docs site:** `website/` is a pure VitePress project; `.github/workflows/pages.yml` builds on every push that touches `website/**` and publishes to `clintmod.github.io/rite/` via GitHub Pages (Source: GitHub Actions — enabled via `gh api POST /repos/clintmod/rite/pages`). Custom domain can be added later by dropping `DOCS_BASE=/rite/` from `pages.yml`.
- **CI:** Test (Go 1.25+1.26 × ubuntu+macos+windows), Lint (golangci-lint v2.11.4, config at `.golangci.yml`), Docs, goreleaser. All green on main at session end.
- **Secrets on `clintmod/rite`:** `GH_PAT` (fine-grained, scoped to `clintmod/homebrew-tap`, exposed to workflow as `HOMEBREW_TAP_TOKEN`).

---

## Known sharp edges

- **Go package is still named `task`.** The module path was renamed to `github.com/clintmod/rite` in Phase 1 but the Go package identifier stayed `task`. Several files that import the root package use an explicit alias: `task "github.com/clintmod/rite"`. goimports / golangci-lint enforce this — don't remove the alias without renaming the package everywhere.
- **`go install` binaries report the wrong version.** goreleaser's ldflags inject `internal/version.version=v0.1.0` for release builds only. A bare `go install github.com/clintmod/rite/cmd/rite@v0.1.0` embeds the fallback constant `3.49.1`. Brew, ubi, and release archives all report the correct version.
- **VitePress + Shiki + Vue bug:** multiple `{{…}}` on one line inside YAML fenced blocks can confuse the Vue SFC compiler. If you see a build error like `src/foo.md (L:C): Error parsing JavaScript expression` and the position doesn't match your source, convert the example to `${VAR}` form (SPEC-preferred anyway). Inline prose references use `<span v-pre>`{{.VAR}}`</span>` as the escape.
- **Older mise + `ubi:` backend strips `v` prefixes.** Users on mise < 2026.4 hitting `"ubi:clintmod/rite" = "v0.1.0"` get a 404. Docs steer them to the `go:` backend fallback. When bumping mise in CI, use 2026.4.x+.
- **`lint-jsonschema` job is gone until we publish a schema.** The upstream workflow validated `website/src/public/schema.json` (Taskfile schema). We deleted the file with the site rebuild; re-add the job when we host our own at `clintmod.github.io/rite/schema.json`.
- **Remote-taskfile experiment (`RITE_X_REMOTE_TASKFILES=1`)** inherits upstream code paths we haven't audited. Docs note this. Don't assume production-grade behavior.

---

## Context locations outside this repo

- **Design contract (editable master):** `~/Dropbox/brain/10-projects/rite/SPEC.md` (Obsidian vault)
- **Decision records (ADRs):** `~/Dropbox/brain/10-projects/rite/decisions/`
- **Work logs and session notes:** `~/Dropbox/brain/10-projects/rite/` (daily notes link in from `~/Dropbox/brain/daily/`)
- **Auto-memory:** `~/.claude/projects/-Users-clint-code-github-clintmod-rite/memory/` (feedback and refs specific to this repo)

When there is a conflict between an ADR/decision and current code behavior, the code is the truth — update the ADR to note the divergence and why.

---

## Conventions

- **Commits:** Prefix with the phase when relevant (`rebrand:`, `precedence:`, `migrate:`). No emoji. Explain the *why*, not just the *what*.
- **No partial rebrands.** Any rename/restructure should land in a single atomic commit across the codebase so the tree is always buildable.
- **SPEC before code.** Behavioral changes that touch variable semantics require a SPEC update first or in the same commit.
- **Don't silently pull from upstream.** Every cherry-pick needs an explicit `git cherry-pick -x` (records the source) and a note in the commit message about why we want it.
