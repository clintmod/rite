# Cleanup with `defer`

`defer:` registers a cmd (or another task) to run **after** the task's main cmds — on success *and* on failure. Use it for teardown that has to happen regardless of how the task ends.

## Basics

```yaml
version: '3'

tasks:
  test-with-cleanup:
    cmds:
      - defer: rm -rf ./tmp
      - mkdir -p ./tmp
      - go test -tmpdir=./tmp ./...
```

The `rm -rf` runs after the `go test`, even if the tests fail. Same idea as Go's `defer` keyword — register cleanup at the moment you allocate the resource, not at the bottom of the function.

## Multiple defers

`defer:` entries are stacked LIFO, like Go's defer. The last-registered runs first:

```yaml
tasks:
  pipeline:
    cmds:
      - defer: echo 'last'
      - defer: echo 'middle'
      - defer: echo 'first'
      - echo 'main work'
```

Output:
```
main work
first
middle
last
```

## Calling another task as defer

The full call form works too — handy when cleanup is itself a non-trivial task:

```yaml
tasks:
  load-test:
    cmds:
      - defer:
          task: cleanup
          vars:
            ENV: '${TARGET}'
      - ./load-gen.sh ${TARGET}

  cleanup:
    cmds:
      - kubectl delete -n ${ENV} -f ./loadgen.yaml
      - aws s3 rm s3://load-results/${ENV} --recursive
```

`vars:` on the deferred call are forwarded as if you'd written `task: cleanup` in the normal cmd position. They land at the callee's CLI tier (tier 2 in [precedence](/precedence)).

## Silent defers

Defers respect the same `silent:` flag as regular cmds. Useful for cleanup whose output isn't relevant unless it fails:

```yaml
tasks:
  ci:
    cmds:
      - defer: docker logout
        silent: true
      - docker login -u $USER -p $PASS
      - docker push myimage:latest
```

## On failure

A failure in the main cmds doesn't stop defers from running. A failure in a *defer itself* doesn't stop subsequent defers from running either — they all run, and rite reports the highest-priority failure (main-cmd failures take priority over defer failures).

If you need a defer to bail loudly when it fails, don't use `defer:` — restructure the task so cleanup is its own step that can fail the parent.

## Differences from go-task

Identical to upstream — `defer:` is unaffected by first-in-wins. The deferred call's `vars:` follow the same precedence rules as any other call site, so shell env still wins.
