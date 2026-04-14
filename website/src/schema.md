---
title: Ritefile Schema Reference
description: A reference for the Ritefile schema
outline: deep
---

# Ritefile Schema Reference

This page documents every property of the Ritefile schema (version 3) and the types it accepts. The shape is near-identical to go-task's Taskfile v3 schema; the *semantics* are where rite differs — see [precedence](/precedence) for the eight-tier variable model and [migration](/migration) for the five user-visible breaks.

> **JSON schema URLs.** Two copies are published; pick based on how much
> churn you're willing to tolerate:
>
> - `https://clintmod.github.io/rite/schema/v3.json` — **pin this.**
>   Frozen at the v3 schema contract. When rite eventually ships a v4
>   schema, `/schema/v3.json` stays exactly where it is so your editor
>   keeps validating v3 Ritefiles correctly.
> - `https://clintmod.github.io/rite/schema.json` — always latest. Good
>   for "just want syntax highlighting" or exploratory use, but no
>   stability guarantee across rite releases.
>
> Point your YAML language server at the versioned URL with a modeline
> at the top of your Ritefile:
>
> ```yaml
> # yaml-language-server: $schema=https://clintmod.github.io/rite/schema/v3.json
> ```
>
> Schema versions are independent of the rite app version — rite may
> ship releases without re-publishing the schema, and vice versa. Runtime
> rules the schema can't express (e.g. "included-file top-level vars
> follow first-in-wins") are documented here and enforced by the binary.

## Root Schema

The root Ritefile schema defines the structure of your main `Ritefile.yml`.

### `version`

- **Type**: `string` or `number`
- **Required**: Yes
- **Valid values**: `"3"`, `3`, or any valid semver string
- **Description**: Version of the Ritefile schema

```yaml
version: '3'
```

### `output`

- **Type**: `string` or `object`
- **Default**: `interleaved`
- **Options**: `interleaved`, `group`, `prefixed`
- **Description**: Controls how task output is displayed

```yaml
# Simple string format
output: group

# Advanced object format
output:
  group:
    begin: "::group::{{.RITE_NAME}}"
    end: "::endgroup::"
    error_only: false
```

### `method`

- **Type**: `string`
- **Default**: `checksum`
- **Options**: `checksum`, `timestamp`, `none`
- **Description**: Default method for checking if tasks are up-to-date

```yaml
method: timestamp
```

### [`includes`](#include)

- **Type**: `map[string]Include`
- **Description**: Include other Ritefiles

```yaml
includes:
  # Simple string format
  docs: ./Ritefile.yml

  # Full object format
  backend:
    taskfile: ./backend
    dir: ./backend
    optional: false
    flatten: false
    internal: false
    aliases: [api]
    excludes: [internal-task]
    vars:
      SERVICE_NAME: backend
    checksum: abc123...
```

### [`vars`](#variable)

- **Type**: `map[string]Variable`
- **Description**: Global variables available to all tasks

```yaml
vars:
  # Simple values
  APP_NAME: myapp
  VERSION: 1.0.0
  DEBUG: true
  PORT: 8080
  FEATURES: [auth, logging]

  # Dynamic variables
  COMMIT_HASH:
    sh: git rev-parse HEAD

  # Variable references
  BUILD_VERSION:
    ref: .VERSION

  # Map variables
  CONFIG:
    map:
      database: postgres
      cache: redis
```

### `env`

- **Type**: `map[string]Variable`
- **Description**: Global environment variables

```yaml
env:
  NODE_ENV: production
  DATABASE_URL:
    sh: echo $DATABASE_URL
```

### [`tasks`](#task)

- **Type**: `map[string]Task`
- **Description**: Task definitions

```yaml
tasks:
  # Simple string format
  hello: echo "Hello World"

  # Array format
  build:
    - go mod tidy
    - go build ./...

  # Full object format
  deploy:
    desc: Deploy the application
    cmds:
      - ./scripts/deploy.sh
```

### `silent`

- **Type**: `bool`
- **Default**: `false`
- **Description**: Suppress task name and command output by default

```yaml
silent: true
```

### `dotenv`

- **Type**: `[]string`
- **Description**: Load environment variables from .env files. When the same
  variable is defined in multiple files, the first file in the list takes
  precedence.

```yaml
dotenv:
  - .env.local # Highest priority
  - .env # Lowest priority
```

### `run`

- **Type**: `string`
- **Default**: `always`
- **Options**: `always`, `once`, `when_changed`
- **Description**: Default execution behavior for tasks

```yaml
run: once
```

### `interval`

- **Type**: `string`
- **Default**: `100ms`
- **Pattern**: `^[0-9]+(?:m|s|ms)$`
- **Description**: Watch interval for file changes

```yaml
interval: 1s
```

### `set`

- **Type**: `[]string`
- **Options**: `allexport`, `a`, `errexit`, `e`, `noexec`, `n`, `noglob`, `f`,
  `nounset`, `u`, `xtrace`, `x`, `pipefail`
- **Description**: POSIX shell options for all commands

```yaml
set: [errexit, nounset, pipefail]
```

### `shopt`

- **Type**: `[]string`
- **Options**: `expand_aliases`, `globstar`, `nullglob`
- **Description**: Bash shell options for all commands

```yaml
shopt: [globstar]
```

## Include

Configuration for including external Ritefiles.

### `taskfile`

- **Type**: `string`
- **Required**: Yes
- **Description**: Path to the Ritefile or directory to include

```yaml
includes:
  backend: ./backend/Ritefile.yml
  # Shorthand for above
  frontend: ./frontend
```

### `dir`

- **Type**: `string`
- **Description**: Working directory for included tasks

```yaml
includes:
  api:
    taskfile: ./api
    dir: ./api
```

### `optional`

- **Type**: `bool`
- **Default**: `false`
- **Description**: Don't error if the included file doesn't exist

```yaml
includes:
  optional-tasks:
    taskfile: ./optional.yml
    optional: true
```

### `flatten`

- **Type**: `bool`
- **Default**: `false`
- **Description**: Include tasks without namespace prefix

```yaml
includes:
  common:
    taskfile: ./common.yml
    flatten: true
```

### `internal`

- **Type**: `bool`
- **Default**: `false`
- **Description**: Hide included tasks from command line and `--list`

```yaml
includes:
  internal:
    taskfile: ./internal.yml
    internal: true
```

### `aliases`

- **Type**: `[]string`
- **Description**: Alternative names for the namespace

```yaml
includes:
  database:
    taskfile: ./db.yml
    aliases: [db, data]
```

### `excludes`

- **Type**: `[]string`
- **Description**: Tasks to exclude from inclusion

```yaml
includes:
  shared:
    taskfile: ./shared.yml
    excludes: [internal-setup, debug-only]
```

### `vars`

- **Type**: `map[string]Variable`
- **Description**: Variables to pass to the included Ritefile

```yaml
includes:
  deploy:
    taskfile: ./deploy.yml
    vars:
      ENVIRONMENT: production
```

### `checksum`

- **Type**: `string`
- **Description**: Expected checksum of the included file

```yaml
includes:
  shared:
    taskfile: ./vendored/shared.yml
    checksum: c153e97e0b3a998a7ed2e61064c6ddaddd0de0c525feefd6bba8569827d8efe9
```

## Variable

Variables support multiple types and can be static values, dynamic commands,
references, or maps.

### Static Variables

```yaml
vars:
  # String
  APP_NAME: myapp
  # Number
  PORT: 8080
  # Boolean
  DEBUG: true
  # Array
  FEATURES: [auth, logging, metrics]
  # Null
  OPTIONAL_VAR: null
```

### Dynamic Variables (`sh`)

```yaml
vars:
  COMMIT_HASH:
    sh: git rev-parse HEAD
  BUILD_TIME:
    sh: date -u +"%Y-%m-%dT%H:%M:%SZ"
```

### Variable References (`ref`)

```yaml
vars:
  BASE_VERSION: 1.0.0
  FULL_VERSION:
    ref: .BASE_VERSION
```

### Map Variables (`map`)

```yaml
vars:
  CONFIG:
    map:
      database:
        host: localhost
        port: 5432
      cache:
        type: redis
        ttl: 3600
```

### Variable Ordering

Variables can reference previously defined variables:

```yaml
vars:
  GREETING: Hello
  TARGET: World
  MESSAGE: '{{.GREETING}} {{.TARGET}}!'
```

## Task

Individual task configuration with multiple syntax options.

### Simple Task Formats

```yaml
tasks:
  # String command
  hello: echo "Hello World"

  # Array of commands
  build:
    - go mod tidy
    - go build ./...

  # Object with cmd shorthand
  test:
    cmd: go test ./...
```

### Task Properties

#### `cmds`

- **Type**: `[]Command`
- **Description**: Commands to execute

```yaml
tasks:
  build:
    cmds:
      - go build ./...
      - echo "Build complete"
```

#### `cmd`

- **Type**: `string`
- **Description**: Single command (alternative to `cmds`)

```yaml
tasks:
  test:
    cmd: go test ./...
```

#### `deps`

- **Type**: `[]Dependency`
- **Description**: Tasks to run before this task

```yaml
tasks:
  # Simple dependencies
  deploy:
    deps: [build, test]
    cmds:
      - ./deploy.sh

  # Dependencies with variables
  advanced-deploy:
    deps:
      - task: build
        vars:
          ENVIRONMENT: production
      - task: test
        vars:
          COVERAGE: true
    cmds:
      - ./deploy.sh

  # Silent dependencies
  main:
    deps:
      - task: setup
        silent: true
    cmds:
      - echo "Main task"

  # Loop dependencies
  test-all:
    deps:
      - for: [unit, integration, e2e]
        task: test
        vars:
          TEST_TYPE: '{{.ITEM}}'
    cmds:
      - echo "All tests completed"
```

#### `desc`

- **Type**: `string`
- **Description**: Short description shown in `--list`

```yaml
tasks:
  test:
    desc: Run unit tests
    cmds:
      - go test ./...
```

#### `summary`

- **Type**: `string`
- **Description**: Detailed description shown in `--summary`

```yaml
tasks:
  deploy:
    desc: Deploy to production
    summary: |
      Deploy the application to production environment.
      This includes building, testing, and uploading artifacts.
```

#### `prompt`

- **Type**: `string` or `[]string`
- **Description**: Prompts shown before task execution

```yaml
tasks:
  # Single prompt
  deploy:
    prompt: "Deploy to production?"
    cmds:
      - ./deploy.sh

  # Multiple prompts
  deploy-multi:
    prompt:
      - "Are you sure?"
      - "This will affect live users!"
    cmds:
      - ./deploy.sh
```

#### `aliases`

- **Type**: `[]string`
- **Description**: Alternative names for the task

```yaml
tasks:
  build:
    aliases: [compile, make]
    cmds:
      - go build ./...
```

#### `method`

- **Type**: `string`
- **Default**: `checksum`
- **Options**: `checksum`, `timestamp`, `none`
- **Description**: Method for checking if the task is up-to-date. Refer to the `method` root property for details.

```yaml
tasks:
  build:
    sources:
      - go.mod
    method: timestamp
```

#### `sources`

- **Type**: `[]string` or `[]Glob`
- **Description**: Source files to monitor for changes

```yaml
tasks:
  build:
    sources:
      - '**/*.go'
      - go.mod
      # With exclusions
      - exclude: '**/*_test.go'
    cmds:
      - go build ./...
```

#### `generates`

- **Type**: `[]string` or `[]Glob`
- **Description**: Files generated by this task

```yaml
tasks:
  build:
    sources: ['**/*.go']
    generates:
      - './app'
      - exclude: '*.debug'
    cmds:
      - go build -o app ./cmd
```

#### `status`

- **Type**: `[]string`
- **Description**: Commands to check if task should run

```yaml
tasks:
  install-deps:
    status:
      - test -f node_modules/.installed
    cmds:
      - npm install
      - touch node_modules/.installed
```

#### `preconditions`

- **Type**: `[]Precondition`
- **Description**: Conditions that must be met before running

```yaml
tasks:
  # Simple precondition (shorthand)
  build:
    preconditions:
      - test -d ./src
    cmds:
      - go build ./...

  # Preconditions with custom messages
  deploy:
    preconditions:
      - sh: test -n "$API_KEY"
        msg: 'API_KEY environment variable is required'
      - sh: test -f ./app
        msg: "Application binary not found. Run 'task build' first."
    cmds:
      - ./deploy.sh
```

#### `if`

- **Type**: `string`
- **Description**: Shell command to conditionally execute the task. If the
  command exits with a non-zero code, the task is skipped (not failed).

```yaml
tasks:
  # Task only runs in CI environment
  deploy:
    if: '[ "$CI" = "true" ]'
    cmds:
      - ./deploy.sh

  # Using Go template expressions
  build-prod:
    if: '{{eq .ENV "production"}}'
    cmds:
      - go build -ldflags="-s -w" ./...
```

#### `dir`

- **Type**: `string`
- **Description**: The directory in which this task should run
- **Default**: If the task is in the root Ritefile, the default `dir` is
  `ROOT_DIR`. For included Ritefiles, the default `dir` is the value specified in
  their respective `includes.*.dir` field (if any).

```yaml
tasks:
  current-dir:
    dir: '{{.USER_WORKING_DIR}}'
    cmd: pwd
```

#### `requires`

- **Type**: `Requires`
- **Description**: Required variables with optional enum validation

```yaml
tasks:
  deploy:
    requires:
      vars: [API_KEY, ENVIRONMENT]
    cmds:
      - ./deploy.sh

  advanced-deploy:
    requires:
      vars:
        - API_KEY
        - name: ENVIRONMENT
          enum: [development, staging, production]
        - name: LOG_LEVEL
          enum: [debug, info, warn, error]
    cmds:
      - echo "Deploying to {{.ENVIRONMENT}} with log level {{.LOG_LEVEL}}"
      - ./deploy.sh


  # Requirements with enum from variable reference
  reusable-deploy:
    requires:
      vars:
        - name: ENVIRONMENT
          enum:
            ref: .ALLOWED_ENVS
    cmds:
      - ./deploy.sh
```

See [the SPEC](https://github.com/clintmod/rite/blob/main/SPEC.md)
for information on enabling interactive prompts for missing required variables.

#### `watch`

- **Type**: `bool`
- **Default**: `false`
- **Description**: Automatically run task in watch mode

```yaml
tasks:
  dev:
    watch: true
    cmds:
      - npm run dev
```

#### `platforms`

- **Type**: `[]string`
- **Description**: Platforms where this task should run

```yaml
tasks:
  windows-build:
    platforms: [windows]
    cmds:
      - go build -o app.exe ./cmd

  unix-build:
    platforms: [linux, darwin]
    cmds:
      - go build -o app ./cmd
```

#### `label`

- **Type**: `string`
- **Description**: Override the display name shown in logs, `--list`, and the
  `RITE_NAME` special var. The canonical name (map key) is unchanged — only
  what the user sees. Useful when a task key is terse but you want a
  human-readable label. See [Label override](/label).

```yaml
tasks:
  build-prod:
    label: 'Production build (release profile)'
    cmds:
      - go build -ldflags='-s -w' ./...
```

#### `vars`

- **Type**: `map[string]Variable`
- **Description**: Task-scoped variables. Under rite's first-in-wins
  precedence, these are defaults — shell env, CLI args, and include-site
  vars can override them. See [precedence](/precedence) for the full
  ordering.

```yaml
tasks:
  deploy:
    vars:
      ENVIRONMENT: staging
      REPLICAS: 3
    cmds:
      - ./deploy.sh ${ENVIRONMENT} ${REPLICAS}
```

#### `env`

- **Type**: `map[string]Variable`
- **Description**: Environment variables exported to the task's shell.
  Same precedence rules as `vars`. Accepts dynamic (`sh:`) and reference
  (`ref:`) forms just like globals.

```yaml
tasks:
  integration-test:
    env:
      DATABASE_URL: postgres://localhost/test
      LOG_LEVEL: debug
    cmds:
      - go test -tags=integration ./...
```

#### `dotenv`

- **Type**: `[]string`
- **Description**: Load environment variables from one or more `.env` files
  before running the task. First file in the list wins on collision. Entries
  declared directly on the entrypoint (root-level `env:` / `dotenv:`) take
  precedence — rite emits a `DOTENV-ENTRY` warning during `migrate` when a
  task-level dotenv key collides with an entrypoint declaration.

```yaml
tasks:
  deploy:
    dotenv:
      - .env.deploy.local  # highest priority
      - .env.deploy        # lower priority
    cmds:
      - ./deploy.sh
```

#### `silent`

- **Type**: `bool`
- **Default**: inherits from root `silent:`
- **Description**: Suppress the task's command echo and header. The CLI
  `--silent` / `-s` flag overrides this. See
  [Silent / dry-run / ignore-error](/silent-dry-ignore).

```yaml
tasks:
  ping:
    silent: true
    cmds:
      - curl -fsS https://example.com/health
```

#### `interactive`

- **Type**: `bool`
- **Default**: `false`
- **Description**: Mark the task as needing an attached TTY (stdin/stdout).
  Forces serial execution with other interactive tasks so their prompts
  don't clobber each other. Use for editors, REPLs, or commands that prompt
  for input. See [Interactive cmds](/interactive).

```yaml
tasks:
  shell:
    interactive: true
    cmds:
      - docker exec -it app bash
```

#### `internal`

- **Type**: `bool`
- **Default**: `false`
- **Description**: Hide the task from `--list` and block direct invocation
  from the CLI. Internal tasks are callable only as deps or from `task:`
  references. See [Internal tasks](/internal-tasks).

```yaml
tasks:
  _setup-test-db:
    internal: true
    cmds:
      - ./scripts/reset-testdb.sh

  integration:
    deps: [_setup-test-db]
    cmds:
      - go test ./...
```

#### `ignore_error`

- **Type**: `bool`
- **Default**: `false`
- **Description**: Continue the task even if one of its `cmds` returns a
  non-zero exit code. Applies to every command in the task unless a
  command-level `ignore_error:` overrides it. See
  [Silent / dry-run / ignore-error](/silent-dry-ignore).

```yaml
tasks:
  cleanup:
    ignore_error: true
    cmds:
      - rm -f /tmp/cache-*
      - docker container prune -f
```

#### `run`

- **Type**: `string`
- **Default**: inherits from root `run:` (which defaults to `always`)
- **Options**: `always`, `once`, `when_changed`
- **Description**: How often the task's body runs within a single `rite`
  invocation. `once` is common for expensive setup deps. See
  [Run modes](/run-modes).

```yaml
tasks:
  install-deps:
    run: once
    cmds:
      - go mod download
```

#### `failfast`

- **Type**: `bool`
- **Default**: `false`
- **Description**: When a parallel `for:` loop inside this task's `cmds`
  has any iteration fail, cancel the rest immediately instead of waiting
  for them to finish. Matches the `--failfast` / `-F` CLI flag's default
  behavior for the whole run; task-level `failfast: true` scopes it to
  this task.

```yaml
tasks:
  test-all:
    failfast: true
    cmds:
      - for: [unit, integration, e2e]
        cmd: go test ./{{.ITEM}}/...
```

#### `prefix`

- **Type**: `string`
- **Default**: the task's own name (`TASK` special var)
- **Description**: Line prefix used by the `prefixed` output mode (set via
  root-level `output: prefixed` or the `--output` CLI flag). Ignored for
  other output modes. Supports the same template vars as any other string
  field.

```yaml
output: prefixed

tasks:
  api:
    prefix: 'api'
    cmds:
      - ./start-api.sh

  worker:
    prefix: 'worker'
    cmds:
      - ./start-worker.sh
```

#### `set`

- **Type**: `[]string`
- **Description**: POSIX shell `set` options applied to every command in
  this task. Overrides the root-level `set:`. See
  [Shell options](#shell-options) below for the full list.

```yaml
tasks:
  strict-build:
    set: [errexit, nounset, pipefail]
    cmds:
      - ./scripts/build.sh
```

#### `shopt`

- **Type**: `[]string`
- **Description**: Bash `shopt` options applied to every command in this
  task. Overrides the root-level `shopt:`. See
  [Shell options](#shell-options) below.

```yaml
tasks:
  find-tests:
    shopt: [globstar, nullglob]
    cmds:
      - ls **/*_test.go
```

## Command

Individual command configuration within a task.

### Basic Commands

```yaml
tasks:
  example:
    cmds:
      - echo "Simple command"
      - ls -la
```

### Command Object

```yaml
tasks:
  example:
    cmds:
      - cmd: echo "Hello World"
        silent: true
        ignore_error: false
        platforms: [linux, darwin]
        set: [errexit]
        shopt: [globstar]
```

### Task References

```yaml
tasks:
  example:
    cmds:
      - task: other-task
        vars:
          PARAM: value
        silent: false
```

### Deferred Commands

```yaml
tasks:
  with-cleanup:
    cmds:
      - echo "Starting work"
      # Deferred command string
      - defer: echo "Cleaning up"
      # Deferred task reference
      - defer:
          task: cleanup-task
          vars:
            CLEANUP_MODE: full
```

### For Loops

#### Loop Over List

```yaml
tasks:
  greet-all:
    cmds:
      - for: [alice, bob, charlie]
        cmd: echo "Hello {{.ITEM}}"
```

#### Loop Over Sources/Generates

```yaml
tasks:
  process-files:
    sources: ['*.txt']
    cmds:
      - for: sources
        cmd: wc -l {{.ITEM}}
      - for: generates
        cmd: gzip {{.ITEM}}
```

#### Loop Over Variable

```yaml
tasks:
  process-items:
    vars:
      ITEMS: 'item1,item2,item3'
    cmds:
      - for:
          var: ITEMS
          split: ','
          as: CURRENT
        cmd: echo "Processing {{.CURRENT}}"
```

#### Loop Over Matrix

```yaml
tasks:
  test-matrix:
    cmds:
      - for:
          matrix:
            OS: [linux, windows, darwin]
            ARCH: [amd64, arm64]
        cmd: echo "Testing {{.ITEM.OS}}/{{.ITEM.ARCH}}"
```

#### Loop in Dependencies

```yaml
tasks:
  build-all:
    deps:
      - for: [frontend, backend, worker]
        task: build
        vars:
          SERVICE: '{{.ITEM}}'
```

### Conditional Commands

Use `if` to conditionally execute a command. If the shell command exits with a
non-zero code, the command is skipped.

```yaml
tasks:
  build:
    cmds:
      # Only run in production
      - cmd: echo "Optimizing for production"
        if: '[ "$ENV" = "production" ]'
      # Using Go templates
      - cmd: echo "Feature enabled"
        if: '{{eq .ENABLE_FEATURE "true"}}'
      # Inside for loops (evaluated per iteration)
      - for: [a, b, c]
        cmd: echo "processing {{.ITEM}}"
        if: '[ "{{.ITEM}}" != "b" ]'
```

## Shell Options

### Set Options

Available `set` options for POSIX shell features:

- `allexport` / `a` - Export all variables
- `errexit` / `e` - Exit on error
- `noexec` / `n` - Read commands but don't execute
- `noglob` / `f` - Disable pathname expansion
- `nounset` / `u` - Error on undefined variables
- `xtrace` / `x` - Print commands before execution
- `pipefail` - Pipe failures propagate

```yaml
# Global level
set: [errexit, nounset, pipefail]

tasks:
  debug:
    # Task level
    set: [xtrace]
    cmds:
      - cmd: echo "This will be traced"
        # Command level
        set: [noexec]
```

### Shopt Options

Available `shopt` options for Bash features:

- `expand_aliases` - Enable alias expansion
- `globstar` - Enable `**` recursive globbing
- `nullglob` - Null glob expansion

```yaml
# Global level
shopt: [globstar]

tasks:
  find-files:
    # Task level
    shopt: [nullglob]
    cmds:
      - cmd: ls **/*.go
        # Command level
        shopt: [globstar]
```
