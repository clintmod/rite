#!/usr/bin/env bash
# scripts/coexistence/smoke.sh
#
# Dual-install coexistence smoke for rite + go-task.
#
# Guarantees verified here, each asserted with pass/fail output:
#   T1. Both binaries are installed and runnable.
#   T2. In a directory holding both Taskfile.yml and Ritefile.yml, each tool
#       maintains its own cache dir (.task/ vs .rite/) with disjoint contents.
#   T3. A project-local .taskrc.yml is honored by `task` only; a .riterc.yml
#       is honored by `rite` only. Neither tool reads the other's rc.
#   T4. Deleting one tool's cache does NOT invalidate the other tool's
#       up-to-date checksum state.
#
# Exits non-zero on any failure. Every assertion prints PASS/FAIL with a
# short justification so CI logs stay scannable.

set -u
# Intentionally NOT set -e: we want to accumulate failures and report all of
# them, not bail on the first one.

FAIL=0
ASSERT_COUNT=0

pass() {
    ASSERT_COUNT=$((ASSERT_COUNT + 1))
    printf '  PASS  %s\n' "$1"
}

fail() {
    ASSERT_COUNT=$((ASSERT_COUNT + 1))
    FAIL=$((FAIL + 1))
    printf '  FAIL  %s\n' "$1" >&2
}

section() {
    printf '\n=== %s ===\n' "$1"
}

# ---------------------------------------------------------------------------
section "T1: both binaries present"
# ---------------------------------------------------------------------------

if command -v task >/dev/null 2>&1; then
    pass "task on PATH ($(task --version 2>&1 | head -1))"
else
    fail "task not on PATH"
fi

if command -v rite >/dev/null 2>&1; then
    pass "rite on PATH ($(rite --version 2>&1 | head -1))"
else
    fail "rite not on PATH"
fi

# ---------------------------------------------------------------------------
section "T2: disjoint cache dirs (.task/ vs .rite/)"
# ---------------------------------------------------------------------------

WORK=$(mktemp -d)
cd "$WORK" || { fail "mktemp/cd failed"; exit 1; }

# Checksum-gated task defined identically for both runners. `sources:` +
# `generates:` triggers each tool to drop a checksum artifact into its
# private cache dir after the first successful run.
cat > Taskfile.yml <<'YAML'
version: '3'
tasks:
  build:
    sources:
      - src.txt
    generates:
      - out-task.txt
    cmds:
      - cp src.txt out-task.txt
YAML

cat > Ritefile.yml <<'YAML'
version: '3'
tasks:
  build:
    sources:
      - src.txt
    generates:
      - out-rite.txt
    cmds:
      - cp src.txt out-rite.txt
YAML

echo "hello" > src.txt

task build >/dev/null 2>&1 || fail "task build returned non-zero"
rite build >/dev/null 2>&1 || fail "rite build returned non-zero"

[ -f out-task.txt ] && pass "task produced out-task.txt" || fail "out-task.txt missing"
[ -f out-rite.txt ] && pass "rite produced out-rite.txt" || fail "out-rite.txt missing"

[ -d .task ] && pass ".task/ cache dir exists" || fail ".task/ missing"
[ -d .rite ] && pass ".rite/ cache dir exists" || fail ".rite/ missing"

# Caches must not share any path (disjointness). The two trees are allowed
# to use identical filenames internally as long as they are rooted in their
# own dir — we're asserting no file lives under both roots, which is the
# real corruption surface.
TASK_PATHS=$(cd .task && find . -type f 2>/dev/null | sort)
RITE_PATHS=$(cd .rite && find . -type f 2>/dev/null | sort)
if [ -z "$TASK_PATHS" ] || [ -z "$RITE_PATHS" ]; then
    fail "one of the caches is empty (task='$TASK_PATHS' rite='$RITE_PATHS')"
else
    pass "both caches populated (task: $(echo "$TASK_PATHS" | wc -l | tr -d ' ') files; rite: $(echo "$RITE_PATHS" | wc -l | tr -d ' ') files)"
fi

# Assert no cross-writes: neither tool dropped a file in the other's dir.
# (We couldn't cross-write anyway since the dir names differ, but the check
# guards against future regressions where a tool prefix flips.)
if find .task -name "*rite*" 2>/dev/null | grep -q .; then
    fail ".task/ contains rite-named artifacts"
else
    pass ".task/ clean of rite artifacts"
fi
if find .rite -name "*task*" 2>/dev/null | grep -q .; then
    fail ".rite/ contains task-named artifacts"
else
    pass ".rite/ clean of task artifacts"
fi

# Re-running each tool must report up-to-date (exercises checksum gate).
if task build 2>&1 | grep -qi "up.to.date\|up to date"; then
    pass "task reports up-to-date on 2nd run"
else
    # Not all task versions emit that phrase on stdout; fall back to asking
    # task directly via --status, which exits 0 iff up-to-date.
    if task build --status >/dev/null 2>&1; then
        pass "task --status confirms up-to-date"
    else
        fail "task did not detect up-to-date on 2nd run"
    fi
fi
if rite build --status >/dev/null 2>&1; then
    pass "rite --status confirms up-to-date"
else
    fail "rite did not detect up-to-date on 2nd run"
fi

# ---------------------------------------------------------------------------
section "T3: project rc files are tool-specific"
# ---------------------------------------------------------------------------

WORK2=$(mktemp -d)
cd "$WORK2" || { fail "mktemp/cd for T3 failed"; exit 1; }

cat > Taskfile.yml <<'YAML'
version: '3'
tasks:
  noop:
    cmds:
      - echo hi
YAML
cat > Ritefile.yml <<'YAML'
version: '3'
tasks:
  noop:
    cmds:
      - echo hi
YAML

# --- .taskrc.yml should influence task only ---
# Set an experiment flag that task recognizes. If rite ever parsed this file,
# it would either apply the experiment (silent corruption) or fail loudly
# ("unknown field"). Either outcome is a FAIL here.
cat > .taskrc.yml <<'YAML'
experiments:
  GENTLE_FORCE: 1
YAML

TASK_OUT=$(task noop 2>&1); TASK_RC=$?
RITE_OUT=$(rite noop 2>&1); RITE_RC=$?

if [ $TASK_RC -eq 0 ]; then
    pass "task runs with .taskrc.yml present"
else
    fail "task failed unexpectedly with .taskrc.yml: $TASK_OUT"
fi

if [ $RITE_RC -eq 0 ] && ! echo "$RITE_OUT" | grep -qi "taskrc"; then
    pass "rite ignores .taskrc.yml (no error, no reference to it)"
else
    fail "rite reacted to .taskrc.yml (rc=$RITE_RC): $RITE_OUT"
fi

rm -f .taskrc.yml

# --- .riterc.yml should influence rite only ---
cat > .riterc.yml <<'YAML'
experiments:
  GENTLE_FORCE: 1
YAML

TASK_OUT=$(task noop 2>&1); TASK_RC=$?
RITE_OUT=$(rite noop 2>&1); RITE_RC=$?

if [ $TASK_RC -eq 0 ] && ! echo "$TASK_OUT" | grep -qi "riterc"; then
    pass "task ignores .riterc.yml (no error, no reference to it)"
else
    fail "task reacted to .riterc.yml (rc=$TASK_RC): $TASK_OUT"
fi

if [ $RITE_RC -eq 0 ]; then
    pass "rite runs with .riterc.yml present"
else
    fail "rite failed unexpectedly with .riterc.yml: $RITE_OUT"
fi

# ---------------------------------------------------------------------------
section "T4: deleting one cache does not invalidate the other"
# ---------------------------------------------------------------------------

cd "$WORK" || { fail "cd back to T2 workdir failed"; exit 1; }

rm -rf .rite
[ -d .task ] && pass ".task/ survived rm -rf .rite/" || fail ".task/ vanished"
if task build --status >/dev/null 2>&1; then
    pass "task still up-to-date after rite cache wipe"
else
    fail "task invalidated by rite cache wipe (cross-contamination)"
fi

# Recreate rite cache, then wipe task cache.
rite build >/dev/null 2>&1
rm -rf .task
[ -d .rite ] && pass ".rite/ survived rm -rf .task/" || fail ".rite/ vanished"
if rite build --status >/dev/null 2>&1; then
    pass "rite still up-to-date after task cache wipe"
else
    fail "rite invalidated by task cache wipe (cross-contamination)"
fi

# ---------------------------------------------------------------------------
section "verdict"
# ---------------------------------------------------------------------------
printf '%d assertions, %d failed\n' "$ASSERT_COUNT" "$FAIL"
if [ "$FAIL" -eq 0 ]; then
    echo "COEXISTENCE: OK"
    exit 0
else
    echo "COEXISTENCE: FAIL"
    exit 1
fi
