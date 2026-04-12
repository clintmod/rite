# rite

> An idempotent task runner with Unix-native variable precedence.

**Status: design phase.** Spec is drafted. No `rite` binary yet. See [`SPEC.md`](./SPEC.md).

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

- **Phase 0 (done):** Repo set up, spec drafted.
- **Phase 1:** Rebrand — module path, binary name, file format discovery.
- **Phase 2:** Rewrite `getVariables()` for first-in-wins precedence; add provenance-preserving scope traversal.
- **Phase 3:** Test fixture audit and rewrite.
- **Phase 4:** `vars` / `env` unification, `${VAR}` preprocessor.
- **Phase 5:** `rite migrate` from Taskfiles; docs site; v1.0.0 release.
