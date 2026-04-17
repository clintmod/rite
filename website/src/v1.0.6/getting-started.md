# Getting started

## Install

### Homebrew (macOS / Linux)

```sh
brew install clintmod/tap/rite
```

### mise

```toml
# mise.toml
[tools]
"ubi:clintmod/rite" = "v1.0.6"
```

Then `mise install`. The [ubi backend](https://mise.jdx.dev/dev-tools/backends/ubi.html) downloads the prebuilt binary from the GitHub release — no Go toolchain needed.

> **Older mise (pre-2026.4.x)?** The `ubi:` backend in older versions mangles `v`-prefixed tags. Either upgrade (`mise self-update`) or fall back to the `go:` backend, which works on any version but builds from source:
> ```toml
> [tools]
> "go:github.com/clintmod/rite/cmd/rite" = "1.0.6"
> ```

### From source (Go 1.26+)

```sh
go install github.com/clintmod/rite/cmd/rite@latest
```

### Manual download

Grab a binary from [the releases page](https://github.com/clintmod/rite/releases/latest) — archives for darwin / linux / windows / freebsd × amd64 / arm64 / arm / 386 / riscv64, plus deb / rpm / apk packages.

## Your first Ritefile

```sh
mkdir hello && cd hello
rite --init          # writes Ritefile.yml
```

The generated file:

```yaml
version: '3'

vars:
  GREETING: Hello, world!

tasks:
  default:
    desc: Print a greeting message
    cmds:
      - echo "{{.GREETING}}"
    silent: true
```

Run it:

```sh
rite                 # runs the default task → Hello, world!
```

## Core commands

| Command | What it does |
|---|---|
| `rite <task>` | Run a task |
| `rite` | Run `default` |
| `rite --list` | Show tasks with descriptions |
| `rite --list-all` | Show all tasks, including undescribed |
| `rite --init` | Create a starter `Ritefile.yml` |
| `rite --migrate Taskfile.yml` | Convert a go-task Taskfile to a Ritefile (warnings to stderr) |
| `rite --watch <task>` | Rerun on file changes |

## Passing variables

Three ways, in precedence order (highest wins):

```sh
FOO=bar rite build             # 1. Shell env — tier 1
rite build FOO=bar             # 2. CLI arg   — tier 2
```

For variables declared inside a Ritefile, see the [precedence model](./precedence) for all seven tiers.

## A more interesting example

```yaml
version: '3'
vars:
  IMAGE: myapp
  TAG: "{{.RITE_VERSION}}"

tasks:
  build:
    desc: Build the container image
    cmds:
      - docker build -t ${IMAGE}:${TAG} .

  push:
    desc: Push the image
    deps: [build]
    cmds:
      - docker push ${IMAGE}:${TAG}
```

Note both `${VAR}` and <span v-pre>`{{.VAR}}`</span> reference variables. They're interchangeable — both resolve against the same set with identical precedence. See [syntax](./syntax) for the full rules.

## Next

- [Variable precedence](./precedence) — the seven-tier model, one table.
- [Syntax reference](./syntax) — what goes where in a Ritefile.
- [.riterc config](./riterc) — optional user-config file for CLI defaults and experiments.
- [Migration from go-task](./migration) — what's different and why.
