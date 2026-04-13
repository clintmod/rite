# Task working directory

By default, every cmd runs in the directory where rite was invoked. `dir:` overrides that per task.

## Basics

```yaml
version: '3'

tasks:
  whereami:
    dir: ./build
    cmds:
      - pwd
```

`rite whereami` runs `pwd` inside `./build`, regardless of where you invoked rite from.

## Path resolution

`dir:` paths are resolved relative to the **Ritefile's location**, not to the shell's CWD. That makes the behavior reproducible no matter where you call rite from:

```sh
$ cd /tmp
$ rite -d ~/code/myapp whereami
/Users/clint/code/myapp/build
```

Absolute paths work too: `dir: /var/log/myapp`.

## Templating

`dir:` is a Go template, so it can reference variables — including ones set on the CLI:

```yaml
tasks:
  test:
    dir: '{{.SERVICE}}'
    cmds:
      - go test ./...
```

```sh
rite test SERVICE=api      # runs in ./api
rite test SERVICE=worker   # runs in ./worker
```

The dir is templated *after* variable resolution, so first-in-wins applies — shell env beats CLI which beats Ritefile defaults. See [variable precedence](/precedence).

## Inheritance through deps and `task:` calls

`dir:` is **per-task**, not inherited. A task that calls another task via `cmds: - task: foo` runs `foo` in `foo`'s declared `dir:`, not in the caller's. If `foo` has no `dir:`, it runs in the rite invocation directory.

This is intentional — composability. Library tasks shouldn't accidentally inherit a working directory from the caller they didn't anticipate.

## Built-in directory variables

You can also reach for these instead of (or alongside) `dir:`:

| Var | Value |
|---|---|
| `ROOT_DIR` | Directory containing the entrypoint Ritefile |
| `TASKFILE_DIR` | Directory containing the Ritefile that defined this task (different from ROOT_DIR for included files) |
| `USER_WORKING_DIR` | The shell's CWD when rite was invoked |

Useful when you want absolute references in cmds without committing to a `dir:`:

```yaml
tasks:
  archive:
    cmds:
      - tar -czf {{.USER_WORKING_DIR}}/build.tgz -C {{.ROOT_DIR}} .
```
