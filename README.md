# rite

> An idempotent task runner with Unix-native variable precedence.

**Status: v0.1.0 shipped, pre-1.0.** Binary builds, test suite is green, SPEC's variable precedence model and `${VAR}` shell-native preprocessor are both live. `rite migrate` converts a `Taskfile.yml` to a `Ritefile.yml` and flags anything that changes meaning under the new semantics. Docs site lives at [clintmod.github.io/rite](https://clintmod.github.io/rite/); v1.0.0 still to come. See [`SPEC.md`](./SPEC.md) for the design contract.

## Install

**Homebrew** (macOS / Linux):
```
brew tap clintmod/tap
brew install rite
```

**Install script** (macOS / Linux / FreeBSD / WSL):
```
curl -sSL https://raw.githubusercontent.com/clintmod/rite/main/install.sh | sh -s -- -b ~/bin
```
Downloads the latest release archive, verifies its SHA-256 against `rite_checksums.txt`, and drops `rite` into `~/bin`. Pass a tag as the last argument to pin a version (`… | sh -s -- -b ~/bin v0.1.0`). Default bindir is `./bin` if `-b` is omitted.

**mise**:
```toml
# mise.toml
[tools]
"ubi:clintmod/rite" = "v0.1.0"
```
(Older mise? See [getting-started](https://clintmod.github.io/rite/getting-started#mise) for the `go:` fallback.)

**From source** (Go 1.25+):
```
go install github.com/clintmod/rite/cmd/rite@latest
```

**Binary download**: [releases page](https://github.com/clintmod/rite/releases/latest) — darwin / linux / windows / freebsd × amd64 / arm64 / arm / 386 / riscv64, plus deb / rpm / apk packages.

## Use

```
rite --init                 # writes Ritefile.yml
rite <task>                 # runs a task
rite --list-all             # show all tasks
rite --migrate Taskfile.yml # convert a go-task Taskfile to a Ritefile
```

The five-second mental model: variables are first-in-wins. Shell env beats CLI args beats `Ritefile` defaults. Task-scope `vars:` are defaults only; if any higher tier sets the name, the task value is ignored.

---

## What is this?

`rite` is a task runner in the same space as `make`, `just`, and `go-task` — you describe tasks in a declarative file, and the tool runs them with dependency resolution, parameters, and shell invocation.

The thing that makes `rite` different is how it handles variables. In one sentence: **the value you set closest to the user wins.** Your shell environment overrides everything. Your CLI arguments override the Ritefile. Internal `vars:` blocks declare defaults, not mandates.

This is how Unix has worked for 50 years. It is not how `go-task` works.

## Why does this exist?

`rite` began as a hard fork of [`go-task/task`](https://github.com/go-task/task). The upstream project has a variable model where a task's own `vars:` block overrides CLI arguments and shell environment — the inverse of every Unix precedent. A [decade of bugs](https://github.com/go-task/task/issues/2034) trace to this choice, and the upstream's [proposed redesign](https://github.com/go-task/task/issues/2035) preserves the inversion.

`rite` takes the opposite position: variable precedence should be first-in-wins, scoped sandboxes should be real, and dynamic variable evaluation should be pure.

See [`SPEC.md`](./SPEC.md) for the full design contract, including:

- The 8-tier variable precedence model
- Scoping rules for included Ritefiles
- Dynamic (`sh:`) variable semantics
- `vars` / `env` unification
- Template syntax (`${VAR}` primary, Go-template secondary)
- File format (`Ritefile`)
- Compatibility with `go-task` (none — `rite migrate` converts one-way)

## Relationship to `go-task`

`rite` imports `go-task`'s git history to preserve attribution under its MIT license, but `rite` is not a compatibility fork:

- Different binary (`rite`, not `task`)
- Different file format (`Ritefile`, not `Taskfile.yml`)
- Incompatible variable semantics
- No intention to merge upstream changes that conflict with the SPEC
- One-way migration tool only

The original project is excellent software with a design choice its creators do not want to revisit. `rite` exists for users who want the different choice.

## License

MIT. See [`LICENSE`](./LICENSE). Original copyright © 2016 Andrey Nering; fork contributions © 2026 Clint Modien.

## Roadmap

- [x] **Phase 0:** Repo set up, spec drafted.
- [x] **Phase 1:** Rebrand — module path, binary name, file format discovery.
- [x] **Phase 1.5:** Cosmetic polish — log prefix, error strings, `rite --init`.
- [x] **Phase 2:** First-in-wins `getVariables()`, per-resolution dynamic-var cache.
- [x] **Phase 3:** Test fixture audit and rewrite; include-site var precedence fix.
- [x] **Phase 4:** `${VAR}` preprocessor, `export: false` opt-out, vars/env unified.
- [x] **Phase 5:** `rite migrate` tool, docs site, v0.1.0 release with Homebrew tap + mise support.
- [ ] **Next:** docs expansion to full user-guide parity, JSON schema publication, v1.0.0.

## Migrating from go-task

`rite` is an intentional semantic break from `go-task`, not a drop-in replacement. The five user-visible changes:

1. **Task-scope `vars:` are defaults only.** If the entrypoint sets `FOO`, a task-scope `FOO` is ignored. Upstream's last-in-wins model is inverted.
2. **Task-scope `env:` is also defaults only.** Same rule, applied to env blocks.
3. **Task-level `dotenv:` files don't override entrypoint env.** Same rule.
4. **`vars:` auto-exports to the cmd shell environ.** Add `export: false` on any var holding a secret that shouldn't leak.
5. **Shell env always wins over Ritefile env:** SPEC tier 1 has no opt-out.

Run `rite --migrate <path/to/Taskfile.yml>` and it will: (a) write a `Ritefile.yml` with include-paths rewritten, and (b) emit warnings to stderr for every site where the old and new meanings differ (OVERRIDE-VAR, OVERRIDE-ENV, DOTENV-ENTRY, SECRET-VAR, SCHEMA-URL).
