#!/usr/bin/env bash
# Test harness for bin/release.sh pure functions.
# Sources release.sh with RELEASE_SH_NO_MAIN=1 to skip orchestration entry point.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

export RELEASE_SH_NO_MAIN=1
# shellcheck source=./release.sh
source "$SCRIPT_DIR/release.sh"

failures=0
pass() { printf "  ok   %s\n" "$1"; }
fail() { printf "  FAIL %s: %s\n" "$1" "$2" >&2; failures=$((failures + 1)); }

assert_eq() {
  local actual="$1" expected="$2" name="$3"
  if [[ "$actual" == "$expected" ]]; then
    pass "$name"
  else
    fail "$name" "expected '$expected', got '$actual'"
  fi
}

assert_ok() {
  local name="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    pass "$name"
  else
    fail "$name" "expected success, got failure: $*"
  fi
}

assert_fails() {
  local name="$1"
  shift
  if ! "$@" >/dev/null 2>&1; then
    pass "$name"
  else
    fail "$name" "expected failure, got success: $*"
  fi
}

echo "validate_semver:"
assert_ok    "accepts 0.0.0"        validate_semver "0.0.0"
assert_ok    "accepts 1.2.3"        validate_semver "1.2.3"
assert_ok    "accepts 0.19.0"       validate_semver "0.19.0"
assert_ok    "accepts 12.34.567"    validate_semver "12.34.567"
assert_fails "rejects empty"        validate_semver ""
assert_fails "rejects v-prefixed"   validate_semver "v1.0.0"
assert_fails "rejects two-part"     validate_semver "1.0"
assert_fails "rejects suffix"       validate_semver "1.0.0-rc.1"
assert_fails "rejects alpha"        validate_semver "abc"
assert_fails "rejects leading zero" validate_semver "01.0.0"

echo
echo "verify_version_sync + bump_version_files:"
TMP="$(mktemp -d -t floq-release-test.XXXXXX)"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/frontend"
printf "0.19.0\n" > "$TMP/VERSION"
cat > "$TMP/README.md" <<'EOF'
# Floq

[![Version](https://img.shields.io/badge/version-0.19.0-blue)](VERSION)
[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue)](LICENSE)
EOF
cat > "$TMP/frontend/package.json" <<'EOF'
{
  "name": "frontend",
  "version": "0.19.0",
  "private": true
}
EOF
cat > "$TMP/frontend/package-lock.json" <<'EOF'
{
  "name": "frontend",
  "version": "0.19.0",
  "lockfileVersion": 3,
  "packages": {
    "": {
      "name": "frontend",
      "version": "0.19.0",
      "dependencies": {}
    },
    "node_modules/some-dep": {
      "version": "4.4.4"
    },
    "node_modules/colliding-dep": {
      "version": "0.20.0"
    },
    "node_modules/another-colliding": {
      "version": "0.20.0"
    }
  }
}
EOF

assert_ok    "synced state accepted"      verify_version_sync "$TMP" "0.19.0"

# Drift in README — must be detected
perl -i -pe 's|version-0\.19\.0-blue|version-0.18.0-blue|' "$TMP/README.md"
assert_fails "drift detected (README)"    verify_version_sync "$TMP" "0.19.0"
perl -i -pe 's|version-0\.18\.0-blue|version-0.19.0-blue|' "$TMP/README.md"

# Drift in package.json — must be detected
perl -i -pe 's|"version": "0\.19\.0"|"version": "0.18.0"|' "$TMP/frontend/package.json"
assert_fails "drift detected (pkg.json)"  verify_version_sync "$TMP" "0.19.0"
perl -i -pe 's|"version": "0\.18\.0"|"version": "0.19.0"|' "$TMP/frontend/package.json"

# Drift in package-lock.json (only root, lockfile gets 1/2 — must fail)
perl -i -pe 'if (/"version": "0\.19\.0"/ && !$done) { s|"version": "0\.19\.0"|"version": "0.18.0"|; $done = 1 }' "$TMP/frontend/package-lock.json"
assert_fails "drift detected (pkg-lock half)" verify_version_sync "$TMP" "0.19.0"
perl -i -pe 's|"version": "0\.18\.0"|"version": "0.19.0"|' "$TMP/frontend/package-lock.json"

# Bump
assert_ok    "bump runs"                  bump_version_files "$TMP" "0.19.0" "0.20.0"
assert_ok    "post-bump sync at 0.20.0"   verify_version_sync "$TMP" "0.20.0"
assert_eq    "$(cat "$TMP/VERSION")" "0.20.0" "VERSION rewritten"
assert_eq    "$(grep -c 'version-0.20.0-blue' "$TMP/README.md")" "1" "README badge updated"
assert_eq    "$(grep -c '0.19.0' "$TMP/README.md")" "0" "README no stale 0.19.0"
assert_eq    "$(grep -c '"version": "0.20.0"' "$TMP/frontend/package.json")" "1" "pkg.json updated"
assert_eq    "$(grep -c '"version": "0.20.0"' "$TMP/frontend/package-lock.json")" "4" "pkg-lock.json 4 occurrences (2 project + 2 colliding deps)"
# Ensure unrelated dep version untouched
assert_eq    "$(grep -c '"version": "4.4.4"' "$TMP/frontend/package-lock.json")" "1" "unrelated dep version preserved"
# Ensure colliding-dep versions NOT bumped (they happen to equal new project version, but they're transitive deps)
assert_eq    "$(grep -c 'colliding-dep' "$TMP/frontend/package-lock.json")" "1" "colliding-dep key preserved"
assert_eq    "$(grep -c 'another-colliding' "$TMP/frontend/package-lock.json")" "1" "another-colliding key preserved"
# The colliding deps started at 0.20.0 and must STILL be at 0.20.0 (we want stability — bump must not invent versions).
# What we cannot accept: the bump removing their 0.20.0 entries or otherwise corrupting them.
# Structural rewrite contract: only root "version" and packages[""].version get touched.
# Verify by reconstructing expected file and diffing whole file content.
cat > "$TMP/frontend/expected-package-lock.json" <<'EOF'
{
  "name": "frontend",
  "version": "0.20.0",
  "lockfileVersion": 3,
  "packages": {
    "": {
      "name": "frontend",
      "version": "0.20.0",
      "dependencies": {}
    },
    "node_modules/some-dep": {
      "version": "4.4.4"
    },
    "node_modules/colliding-dep": {
      "version": "0.20.0"
    },
    "node_modules/another-colliding": {
      "version": "0.20.0"
    }
  }
}
EOF
if diff -q "$TMP/frontend/package-lock.json" "$TMP/frontend/expected-package-lock.json" >/dev/null; then
  pass "pkg-lock.json structural fidelity"
else
  fail "pkg-lock.json structural fidelity" "$(diff "$TMP/frontend/expected-package-lock.json" "$TMP/frontend/package-lock.json")"
fi

# Also: verify_version_sync at the new version must return exactly true when project version
# coincidentally matches transitive dep versions (the magic-count-2 invariant must use structural
# matching, not raw grep count).
assert_ok    "verify_version_sync robust to dep collision" verify_version_sync "$TMP" "0.20.0"

echo
echo "notes_file EXIT trap (issue #70):"
# The release.sh main() registers an EXIT trap to clean up the temp notes
# file. The trap is set INSIDE main() where notes_file is `local`. Under
# `set -u`, if the trap body defers the variable lookup until fire time
# (after main returns and the local is out of scope), the script exits
# with "notes_file: unbound variable" — observed in production on
# v0.24.1 release.
#
# This test replays the EXACT trap line from release.sh in an isolated
# subshell so the regression is caught at the source.
NOTES_TRAP_LINE="$(grep -E "^  trap [\"'].*rm " "$SCRIPT_DIR/release.sh" | head -1)"
[[ -n "$NOTES_TRAP_LINE" ]] || { echo "ERROR: could not locate trap line in release.sh" >&2; exit 1; }

NOTES_TMP="$(mktemp -t floq-notes-trap.XXXXXX)"
mv -- "$NOTES_TMP" "${NOTES_TMP}.md"
NOTES_TMP="${NOTES_TMP}.md"

# Reproduce the main()-shaped scope: notes_file is local; trap is set;
# function returns; script exits.
notes_stderr="$(
  bash -c "
set -uo pipefail
f() {
  local notes_file='$NOTES_TMP'
  $NOTES_TRAP_LINE
}
f
" 2>&1
)"

if [[ -z "$notes_stderr" ]]; then
  pass "no unbound variable after function returns"
else
  fail "no unbound variable after function returns" "stderr: $notes_stderr"
fi

if [[ ! -e "$NOTES_TMP" ]]; then
  pass "trap removes notes_file on exit"
else
  fail "trap removes notes_file on exit" "file still exists: $NOTES_TMP"
  rm -f "$NOTES_TMP"
fi

echo
if [[ "$failures" -gt 0 ]]; then
  printf "FAIL: %d test(s) failed\n" "$failures" >&2
  exit 1
fi
echo "All tests passed."
