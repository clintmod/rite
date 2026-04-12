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

**Run the project's own tasks** (eventually — once tests are fixed in Phase 2):
```bash
rite lint
rite test
```

**File discovery:** `Ritefile`, `Ritefile.yml`, `Ritefile.yaml`, `Ritefile.dist.yml`, `Ritefile.dist.yaml` (plus lowercase variants). No Taskfile recognition.

**Env var prefix:** `RITE_*` for rite-internal config. `RITE_X_*` for experiment flags (e.g. `RITE_X_ENV_PRECEDENCE=1`).

---

## Phase status

- **Phase 0 (done):** Repo identity — SPEC, LICENSE, NOTICE, README.
- **Phase 1 (done, commit `bebe02bc`):** Rebrand — module path, binary, file discovery, env vars, special vars. Still uses upstream's variable precedence semantics.
- **Phase 1.5 (deferred):** Cosmetic polish — log prefix (`task:` → `rite:`), error message strings, user-visible docs.
- **Phase 2 (next, the point of the project):** Rewrite `compiler.go:getVariables()` for first-in-wins precedence. Replace merging with DAG-walk resolution. Make dynamic `sh:` vars per-resolution.
- **Phase 3:** Test fixture audit and rewrite (~180 fixtures, ~40-80% rely on last-in-wins semantics).
- **Phase 4:** `vars`/`env` unification, `${VAR}` shell-syntax preprocessor.
- **Phase 5:** `rite migrate` tool; docs site; v1.0.0.

---

## Known sharp edges

- **Log prefix still prints `task: [default]` instead of `rite:`.** Cosmetic. Phase 1.5.
- **Many tests currently fail.** They assert on `TASK_*` special var names or "Taskfile" substrings in error output. Don't fix piecemeal — they'll be rewritten wholesale in Phase 3 against the new semantics.
- **`website/` docs are unrebranded.** Vuepress content still references `taskfile.dev` and Taskfile syntax. Do not touch unless working on Phase 5 docs.
- **Variable precedence is currently upstream's broken model.** Shell env does NOT override task-scope `vars:`. This is what Phase 2 exists to fix. Don't be surprised.

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
