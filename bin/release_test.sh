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
assert_eq    "$(grep -c '"version": "0.20.0"' "$TMP/frontend/package-lock.json")" "2" "pkg-lock.json 2 occurrences"
# Ensure unrelated dep version untouched
assert_eq    "$(grep -c '"version": "4.4.4"' "$TMP/frontend/package-lock.json")" "1" "unrelated dep version preserved"

echo
if [[ "$failures" -gt 0 ]]; then
  printf "FAIL: %d test(s) failed\n" "$failures" >&2
  exit 1
fi
echo "All tests passed."
