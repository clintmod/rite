# For-loops and matrices

Repeat a cmd (or a dep) over a list of items. rite expands the list at runtime, producing one command per item.

## Over a literal list

```yaml
tasks:
  build-all:
    cmds:
      - for: [darwin, linux, windows]
        cmd: GOOS=${ITEM} go build -o bin/app-${ITEM} ./cmd/app
```

Each item becomes `${ITEM}` in the templated cmd.

Rename the iterator:
```yaml
cmds:
  - for:
      list: [darwin, linux, windows]
      as: OS
    cmd: GOOS=${OS} go build
```

## Over a variable

```yaml
vars:
  SERVICES: "api web worker"

tasks:
  restart:
    cmds:
      - for:
          var: SERVICES
        cmd: systemctl restart ${ITEM}
```

The var gets split on whitespace by default. Custom split:

```yaml
- for:
    var: SERVICES
    split: ","        # comma-separated
  cmd: …
```

If the var is already a list (e.g. populated by `sh:` with `fromJson`), no split needed.

## Over `sources:` / `generates:`

Loop over each file matched by the task's `sources:` or `generates:` glob:

```yaml
tasks:
  lint:
    sources: ['**/*.go']
    cmds:
      - for: sources
        cmd: golangci-lint run ${ITEM}
```

`${ITEM}` is each source file, relative to the task's `dir:`.

## Over a map

When looping over a map (e.g. from a JSON config), both key and value are available:

```yaml
vars:
  HOSTS:
    map:
      api: 10.0.0.1
      web: 10.0.0.2

tasks:
  ping-all:
    cmds:
      - for:
          var: HOSTS
        cmd: "ping ${KEY} ${ITEM}"
```

## Matrix — the cartesian product

```yaml
tasks:
  test-matrix:
    cmds:
      - for:
          matrix:
            GOOS: [darwin, linux, windows]
            GOARCH: [amd64, arm64]
        cmd: GOOS={{.ITEM.GOOS}} GOARCH={{.ITEM.GOARCH}} go test ./...
```

Produces 6 runs (3 × 2). `${ITEM}` is a map of the per-combo values; each axis is a field on `.ITEM`.

Reference an existing list:

```yaml
vars:
  PLATFORMS: [darwin, linux]

tasks:
  build:
    cmds:
      - for:
          matrix:
            GOOS:
              ref: .PLATFORMS
            GOARCH: [amd64, arm64]
        cmd: …
```

## For-loops on deps

`deps:` accept `for:` just like cmds:

```yaml
tasks:
  build-all:
    deps:
      - for:
          list: [api, web, worker]
          as: SERVICE
        task: build
        vars:
          NAME: '${SERVICE}'
    cmds:
      - echo "all three built"
```

Each iteration enqueues a call to `build` with a different `NAME`. Since deps run concurrently, all three builds happen in parallel.

## Empty lists

If the list is empty, the for-loop produces zero commands — the task's other cmds still run. This is how you make a loop optional on a glob that might have no matches:

```yaml
tasks:
  format-generated:
    sources: ['**/*.pb.go']
    cmds:
      - for: sources
        cmd: gofmt -w ${ITEM}
      - echo "done"      # runs even if no pb.go files exist
```
