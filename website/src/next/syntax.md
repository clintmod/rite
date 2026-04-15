# Syntax reference

A Ritefile is YAML. It looks a lot like a Taskfile — the differences are semantic (see [precedence](./precedence)) more than structural.

## Top-level keys

```yaml
version: '3'            # required

vars:                   # entrypoint vars — tier 5
  FOO: bar

env:                    # entrypoint env — same tier as vars, exports to shell
  NODE_ENV: production

dotenv:                 # entrypoint dotenv files — tier 4
  - '.env'
  - '.env.local'

includes:               # other Ritefiles — their tasks join this namespace
  lib:
    taskfile: ./lib/Ritefile.yml
    vars:
      PREFIX: hello     # include-site vars — tier 6

tasks:                  # one or more task definitions
  build: ...
```

## Tasks

```yaml
tasks:
  build:
    desc: Compile the binary
    aliases: [b]
    vars:              # task-scope vars — tier 7, defaults only
      OUT_DIR: ./dist
    env:               # task-scope env — also tier 7, defaults only
      GOFLAGS: -trimpath
    dir: '{{.OUT_DIR}}/..'
    sources: ['**/*.go']
    generates: ['{{.OUT_DIR}}/mybinary']
    cmds:
      - go build -o ${OUT_DIR}/mybinary ./cmd/mybinary
    deps: [test]
    requires:
      vars: [VERSION]
```

## Variable references

Two interchangeable syntaxes, same precedence resolution:

### Shell-native `${VAR}` or `$VAR`

```yaml
cmds:
  - docker build -t myapp:${VERSION} .
  - echo "Deploying to ${ENV}"
```

`${VAR}` and `$VAR` (where `VAR` is `[a-zA-Z_][a-zA-Z0-9_]*`) expand inline against the resolved variable set. Unknown names pass through literal so the shell can still handle `$?`, `$1`, env-only vars, etc.

`$$` escapes to a literal `$`.

See [why the braced form is preferred](#why-braced), [defensive `${VAR:-default}` conventions](#defensive-defaults), and [POSIX quoting rules](#posix-quoting) below.

#### POSIX quoting rules {#posix-quoting}

The preprocessor honors POSIX shell quoting so a Ritefile can emit a literal `$X` (heredoc help text, sed/awk scripts) without sentinels.

| context | example | result |
|---|---|---|
| outside quotes | <span v-pre>`${NAME}`</span> | expanded |
| outside, backslash-escaped | <span v-pre>`\${NAME}`</span> | literal <span v-pre>`${NAME}`</span> |
| double quotes | <span v-pre>`"${NAME}"`</span> | expanded |
| double, backslash-escaped | <span v-pre>`"\${NAME}"`</span> | literal <span v-pre>`${NAME}`</span> |
| single quotes | <span v-pre>`'${NAME}'`</span> | literal <span v-pre>`${NAME}`</span> |
| single, with backslash | <span v-pre>`'\$X'`</span> | literal <span v-pre>`\$X`</span> (POSIX: `\` is literal in `'…'`) |
| heredoc with bare delim | <span v-pre>`<<EOF`</span> body | expanded |
| heredoc with quoted delim | <span v-pre>`<<'EOF'`</span>, <span v-pre>`<<"EOF"`</span>, <span v-pre>`<<\EOF`</span> body | literal |

The `<<-DELIM` (tab-stripping) form follows the same rules; only the `'…'` / `"…"` / `\` decoration on the delimiter changes whether the body expands.

A practical use: emitting installation instructions that mention shell variables verbatim.

```yaml
cmds:
  - |
    cat >&2 <<'HELP'
    Add one of these to your shell rc:
      export PATH="$HOME/.local/bin:$PATH"
    HELP
```

Without quoting honor the preprocessor would substitute `$HOME` and `$PATH` from rite's environment before bash ever saw the heredoc, leaving the user with a confusingly literalized `export PATH="/home/you/.local/bin:/usr/bin:/bin..."` line. The quoted heredoc keeps it as the documentation source intended.

#### Why the braced form is preferred {#why-braced}

rite's preprocessor and the cmd shell both understand `$VAR` — and that's the problem. When both layers claim the same syntax, any name rite doesn't recognize has to be decided at substitution time rather than left to the shell. `${VAR}` avoids the collision:

- **Unambiguous boundaries.** `${VERSION}-rc1` always expands `VERSION`; `$VERSION-rc1` has to guess where the name ends. Shells resolve this one way, rite's preprocessor resolves it the same way, but the reader does the work twice.
- **Preprocessor-only space.** `${NAME}` reads as *"rite resolves this before the shell ever sees it."* `$NAME` in the same document could mean either layer — and usually you *want* the shell layer for things like `$PATH`, `$1`, `$?`, or env-only vars that rite doesn't know about. The braced form carves out rite's slice of the namespace without squatting on the shell's.
- **Grep-ability.** `grep '\${[A-Z_]\+}'` reliably finds every rite-resolved reference. `grep '\$[A-Z_]\+'` also hits `$PATH`, `$HOME`, and every shell-resolved reference, which is rarely what you want.

Both forms work. The bare form exists because some expressions read more cleanly without braces (`echo $HOME`). Reach for `${VAR}` first; drop the braces only when the reference is obviously shell-level.

#### Defensive `${VAR:-default}` conventions {#defensive-defaults}

When `VAR` is set, `${VAR}` and `${VAR:-default}` produce identical output. The `:-default` form pays for itself in three situations:

- **`set -u` resilience.** Tasks that run under `set -u` (strict mode) or `bash -u` abort on any unset reference. `${VAR:-}` (empty default) converts "unset" into "empty string" without touching the happy path where `VAR` is defined.
- **Shellcheck + review signal.** `${VAR:-}` tells the reader *"I thought about what happens when this is missing."* Bare `${VAR}` is ambiguous — either the author guaranteed it's set, or forgot to check. The `:-` form removes the ambiguity.
- **Two-pass resolution.** rite's preprocessor substitutes known names and leaves unknowns for the shell. If `VAR` isn't declared in the Ritefile but *is* in the shell environ, `${VAR}` passes through and the shell handles it. If neither layer knows the name, `${VAR}` expands to empty under most shells but aborts under `set -u`. `${VAR:-default}` survives both.

Rule of thumb: **use `${VAR:-}` for anything optional and `${VAR}` only when the name is required and the task would be broken without it.** Pair required refs with a `requires: vars: [NAME]` task-level declaration so the failure mode is an early clear error instead of an empty string deep inside a cmd.

```yaml
tasks:
  deploy:
    requires:
      vars: [VERSION]        # required — fail fast if missing
    cmds:
      - docker build -t myapp:${VERSION} .                          # required, no default
      - docker push myapp:${VERSION}${TAG_SUFFIX:-}                 # optional suffix, empty default
      - echo "region=${AWS_REGION:-us-west-2}"                      # optional, sensible default
```

### Go template <span v-pre>`{{.VAR}}`</span>

```yaml
sources:
  - "src/**/*.{{.LANG}}"
cmds:
  - echo "building {{.TARGET}}"
```

Go-template syntax carries the full [template function set](https://pkg.go.dev/text/template) — useful for conditionals, list operations, and template functions. A conditional cmd looks like <span v-pre>`"{{if eq .MODE \"release\"}}cargo build --release{{else}}cargo build{{end}}"`</span>; see [`rite/internal/task/testdata/if/Ritefile.yml`](https://github.com/clintmod/rite/blob/main/internal/task/testdata/if/Ritefile.yml) for a working fixture.

**Both syntaxes resolve against the same variable set with identical precedence.** Pick whichever reads cleaner in context — usually `${VAR}` for inline value interpolation, <span v-pre>`{{…}}`</span> for conditionals and pipelines.

## Non-exported variables

By default every declared variable reaches the cmd shell environ. For values that should be visible inside Ritefile logic but not exported:

```yaml
vars:
  INTERNAL_TOKEN:
    value: "abc123"
    export: false
```

Equivalent shorthand for the exported case:
```yaml
FOO: bar
# identical to:
FOO:
  value: bar
  export: true
```

### What can actually be exported

The process environ is a flat list of `KEY=VALUE` strings, so only scalar values reach the cmd shell. Specifically:

- **Scalars** — `string`, `bool`, `int`, `float` — export normally.
- **Structured values** — `map:` vars, lists, `ref:` expressions that resolve to a map — are usable inside Ritefile templating but **silently skipped** when the cmd shell environ is built. Writing `export: true` on a map doesn't produce an error; it's just a no-op.

If you genuinely need a structured value in the child process environ, encode it to a scalar yourself — rite won't pick a flattening convention for you:

```yaml
vars:
  CONFIG:
    map:
      host: api.example.com
      port: 8080
  CONFIG_JSON: '{{toJson .CONFIG}}'   # scalar, exports cleanly
tasks:
  run:
    cmds:
      - echo "$CONFIG_JSON" | jq .host
```

## Dynamic variables

```yaml
vars:
  GIT_SHA:
    sh: git rev-parse --short HEAD
  IS_CLEAN:
    sh: '[ -z "$(git status --porcelain)" ] && echo 1 || echo 0'
```

- Lazily evaluated (only if referenced)
- Per-resolution cache (see [precedence §Dynamic variables](./precedence#dynamic-variables))
- The `sh:` command runs in the task's working directory unless a `dir:` is set on the var

## References

```yaml
vars:
  CONFIG:
    ref: .SOME_MAP.nested.field
```

`ref:` resolves a dotted path against another variable's structure — useful with `sh:` vars that parse JSON or with `map:` vars.

## Commands

```yaml
tasks:
  multi:
    cmds:
      - echo "simple string form"
      - cmd: echo "mapping form, with attributes"
        ignore_error: true
        platforms: [linux, darwin]
      - task: another-task   # call another task inline
        vars:
          ARG: value
      - for: ['a', 'b', 'c']
        cmd: echo processing {{.ITEM}}
      - defer: echo "runs on task exit"
```

## Sources and generates (incremental builds)

```yaml
tasks:
  build:
    sources:
      - 'src/**/*.go'
      - go.mod
    generates:
      - ./bin/app
    method: checksum       # or timestamp, or none
    cmds:
      - go build -o ./bin/app ./cmd/app
```

The task is skipped when every `sources:` file is unchanged since the last `generates:` file was written.

## What's different from go-task

The YAML schema is near-identical; the semantics are the thing rite actually changes. See [the migration guide](./migration) for the five user-visible breaks.
