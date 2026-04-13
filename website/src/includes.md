# Includes

rite supports composing a Ritefile from multiple files. An included Ritefile contributes its tasks into a namespace under the parent, its top-level vars into the include-site scope, and nothing else leaks — each include is a sandbox.

## Shortest form

```yaml
# Ritefile.yml
version: '3'

includes:
  lib: ./lib/Ritefile.yml

tasks:
  build:
    deps: ['lib:generate']
    cmds: ['go build ./...']
```

```yaml
# lib/Ritefile.yml
version: '3'

tasks:
  generate:
    cmds: ['go generate ./...']
```

Tasks from `lib` appear as `lib:<name>` in the parent. The include path is relative to the including Ritefile.

## Advanced form (the mapping)

```yaml
includes:
  svc:
    taskfile: ./services/api/Ritefile.yml
    dir: ./services/api           # run svc:* tasks from this directory
    vars:                         # pass vars into the included file
      SERVICE_NAME: api
    optional: true                # don't error if the file is missing
    internal: false               # prevent svc:* tasks from --list (still callable internally)
    flatten: false                # set true to merge tasks without namespacing
    aliases: [s]                  # rite s:build == rite svc:build
    excludes: [deploy]            # pretend these task names don't exist here
```

See [schema: includes](/schema#includes) for the exhaustive list of keys.

## What variables the included file sees

When you include a file with site-vars, those sit at **tier 6 (include-site vars)** for every task inside that include. Entrypoint vars still win (tier 5), shell env still wins absolutely (tier 1).

```yaml
# Parent Ritefile.yml
vars:
  GLOBAL: from-parent

includes:
  lib:
    taskfile: ./lib
    vars:
      GREETING: hello          # tier 6 for lib's tasks
```

```yaml
# lib/Ritefile.yml
vars:
  GREETING: default            # tier 6 (this file's top vars) — tied to above
  EXTRA: local                 # tier 6, only visible to lib's tasks

tasks:
  greet:
    cmds:
      - echo "${GREETING} — ${GLOBAL} — ${EXTRA}"
```

Running `rite lib:greet`:
- `GREETING` = `hello` (include-site wins within tier 6)
- `GLOBAL` = `from-parent` (parent's entrypoint vars are visible)
- `EXTRA` = `local` (lib's own top vars)

## What doesn't leak

- **Sibling includes cannot see each other.** If the parent includes `lib` and `svc`, `lib`'s vars are invisible to `svc` and vice versa.
- **Included files cannot mutate the parent.** A var declared inside `lib/Ritefile.yml` cannot reach back up.
- **Task-scope `vars:` are task-local.** `lib:foo`'s `vars:` aren't visible to `lib:bar`.

Formally this is SPEC §Scoping. It fixes [go-task/task#2600](https://github.com/go-task/task/issues/2600), [#2680](https://github.com/go-task/task/issues/2680), and [#2108](https://github.com/go-task/task/issues/2108), all of which are symptoms of the same missing sandbox.

## Nested includes

Includes compose transitively — if A includes B, and B includes C, tasks from C appear in A as `b:c:<name>`:

```yaml
# Ritefile.yml in A
includes:
  b: ./b

tasks:
  run: ['rite b:c:build']
```

```yaml
# b/Ritefile.yml
includes:
  c: ./c
```

```yaml
# b/c/Ritefile.yml
tasks:
  build: ['echo building']
```

Each include-site's vars cascade through one level. B's include of C only exposes tier-6 vars to C's tasks; A's vars pass through A → B → C via tier-5 entrypoint scope.

## Interpolated include paths

The `taskfile:` and `dir:` keys accept template syntax, resolved against the parent's variable set at setup time:

```yaml
vars:
  MODULE: api

includes:
  svc:
    taskfile: './services/{{.MODULE}}/Ritefile.yml'
    dir:      './services/{{.MODULE}}'
```

This is useful for factored-out monorepos where the include path is a parameter.

## Remote includes (experiment)

Gated behind the `RITE_X_REMOTE_TASKFILES=1` experiment. Given that gate, you can include from a URL:

```yaml
includes:
  shared:
    taskfile: https://example.com/shared.yml
```

Treat this as alpha — the remote fetch path has upstream quirks we haven't audited for rite. Prefer local includes via a vendored submodule or pinned path for now.
