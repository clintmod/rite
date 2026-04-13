# Platform-specific tasks and commands

Gate a task — or an individual cmd — to specific operating systems or architectures with `platforms:`.

## Whole-task gating

```yaml
version: '3'

tasks:
  build-windows:
    platforms: [windows]
    cmds:
      - echo 'Running task on windows'

  build-darwin:
    platforms: [darwin]
    cmds:
      - echo 'Running task on darwin'
```

On Linux, both tasks **silently skip** — no error, no output. They're effectively absent. This is on purpose: it lets you `deps: [build-darwin, build-linux, build-windows]` from a parent task and have whichever one matches the host run.

Empty `platforms: []` means "always run" — equivalent to omitting the field.

## OS or arch — or both

```yaml
tasks:
  build-amd64:
    platforms: [amd64]   # any amd64 host

  build-arm64:
    platforms: [arm64]   # any arm64 host

  build-mixed:
    cmds:
      - cmd: echo 'darwin'
        platforms: [darwin]
      - cmd: echo 'win/arm64'
        platforms: [windows/arm64]
      - cmd: echo 'linux/amd64'
        platforms: [linux/amd64]
```

Forms accepted:

- `darwin` — match by OS only
- `amd64` — match by arch only
- `darwin/arm64` — both must match

## Per-cmd gating

The same field works on individual cmds, so a single task can branch on platform without splitting itself into three:

```yaml
tasks:
  install-dep:
    cmds:
      - cmd: brew install foo
        platforms: [darwin]
      - cmd: apt-get install -y foo
        platforms: [linux]
      - cmd: choco install foo
        platforms: [windows]
```

Whichever cmd's platform matches runs; the others are skipped.

## Silent skip vs explicit error

A task that skips because of `platforms:` exits 0 from the dependent's perspective — it didn't fail, it didn't run. If you want a "this task shouldn't be invoked on this OS" error instead, use [`requires:` or `preconditions:`](/preconditions) which fail loudly:

```yaml
tasks:
  release-mac:
    preconditions:
      - sh: '[ "$(uname)" = "Darwin" ]'
        msg: "release-mac only runs on macOS"
    cmds:
      - ./build-and-sign-mac.sh
```

Use `platforms:` when *not running* is the right behavior. Use preconditions when *being asked to run* is itself the bug.
