# Wildcard task names

Define a single task whose name contains `*`, and rite matches any invocation that fits the pattern. The captured chunks land in `MATCH` — a list available inside the task's templates.

## Basics

```yaml
version: '3'

tasks:
  start-*:
    desc: Start a service by name
    cmds:
      - echo "Starting service"
```

```sh
$ rite start-api
Starting service

$ rite start-worker
Starting service
```

The pattern matches every name beginning with `start-`. To make the cmds *use* the captured value, you alias `MATCH[N]` into a `vars:` block — see the next section.

## Accessing `MATCH`

`MATCH` is a list of the captured chunks (one per `*`), available inside the task's Go-template context. The standard pattern: alias the captured value through `vars:` so cmds can reference it as a normal `${SERVICE}` (or whatever name fits the domain).

The Go-template form for indexing into `MATCH` is described in [syntax — template helpers](/syntax). The relevant helper is `index`, which takes the list and a zero-based position. A `vars:` alias built from `MATCH[0]` looks like (in pseudocode form here, since the docs site can't safely render the literal Go-template braces inside a YAML fenced block):

> `vars: { SERVICE: <go-template indexing MATCH[0]> }`

In your real Ritefile, you write the value as a quoted Go-template string. A complete worked example lives in [`testdata/wildcards/Ritefile.yml`](https://github.com/clintmod/rite/blob/main/testdata/wildcards/Ritefile.yml) — copy that pattern.

## Multiple wildcards

For task names with more than one `*`, declare one `vars:` alias per capture, indexing element 0, 1, etc. The same caveat applies: use a quoted Go-template string in the `vars:` value, then reference the alias as a shell variable in your cmds:

```yaml
tasks:
  build-*-*:
    desc: Build a component for a given arch (e.g. build-api-linux)
    cmds:
      - echo "Building ${COMPONENT} for ${ARCH}"
```

(With `vars:` aliases for `COMPONENT` and `ARCH` constructed as described above.)

```sh
rite build-api-linux        # COMPONENT=api, ARCH=linux
rite build-worker-arm64     # COMPONENT=worker, ARCH=arm64
```

Wildcards can appear anywhere in the name, including at the start. When they do, quote the YAML key — otherwise YAML parses `*foo` as a node alias:

```yaml
tasks:
  '*-deploy-*':
    cmds:
      - echo "${ENV} deploying ${APP}"
```

## Aliases with wildcards

Wildcards work in `aliases:` too, so you can offer a short form alongside the verbose one:

```yaml
tasks:
  start-*:
    aliases: ['s-*']
    cmds:
      - echo "Starting ${SERVICE}"
```

`rite s-api` and `rite start-api` both work, both produce the same `MATCH`.

## Exact-match callout

If the task name is invoked literally — no wildcard expansion — `MATCH` is empty. A script that shells out `rite "matches-exactly-*"` without expansion will see an empty `MATCH`, and any `vars:` alias built from indexing it will be empty too. Guard with a Go-template length-check if you ever expect the literal form.

## Limitations

- Wildcards match a non-empty sequence of characters. There's no `?` for single-character match, no character classes, no globs.
- The wildcard captures everything between literal segments — no other delimiter rules.
- Wildcards apply to *task names only*. They don't apply to includes' namespaces or to `task:` call sites.
