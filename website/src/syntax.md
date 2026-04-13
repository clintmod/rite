# Syntax reference

A Ritefile is YAML. It looks a lot like a Taskfile — the differences are semantic (see [precedence](/precedence)) more than structural.

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

### Go template <span v-pre>`{{.VAR}}`</span>

```yaml
sources:
  - "src/**/*.{{.LANG}}"
cmds:
  - "{{if eq .MODE \"release\"}}cargo build --release{{else}}cargo build{{end}}"
```

Go-template syntax carries the full [template function set](https://pkg.go.dev/text/template) — useful for conditionals, list operations, and template functions.

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

## Dynamic variables

```yaml
vars:
  GIT_SHA:
    sh: git rev-parse --short HEAD
  IS_CLEAN:
    sh: '[ -z "$(git status --porcelain)" ] && echo 1 || echo 0'
```

- Lazily evaluated (only if referenced)
- Per-resolution cache (see [precedence §Dynamic variables](/precedence#dynamic-variables))
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

The YAML schema is near-identical; the semantics are the thing rite actually changes. See [the migration guide](/migration) for the five user-visible breaks.
