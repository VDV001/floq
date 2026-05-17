#!/usr/bin/env bash
# bin/release.sh X.Y.Z [--dry-run]
#
# Bumps version in all sync points and publishes a release. Sync points:
#   - /VERSION
#   - /README.md shields.io badge
#   - /frontend/package.json   "version"
#   - /frontend/package-lock.json (root + packages[""].version, 2 occurrences)
#
# The script refuses to run unless all four are already in sync at the
# version currently in /VERSION — this catches partial-sync drift before
# layering more on top.
#
# Pure functions are sourced by bin/release_test.sh; orchestration runs
# when invoked as a program (gated by RELEASE_SH_NO_MAIN env var).

set -uo pipefail

# ---------- pure functions (testable) ----------

# validate_semver X.Y.Z (no v prefix, no -suffix). Returns 0 if valid.
validate_semver() {
  local v="${1:-}"
  [[ "$v" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]
}

# verify_version_sync <repo_root> <expected_version>
# Returns 0 iff all four sync points are at expected_version. Uses jq for
# structural matching on the npm JSON files so transitive deps that happen
# to share the version string don't fool the check.
verify_version_sync() {
  local root="$1" expected="$2"
  [[ "$(<"$root/VERSION")" == "$expected" ]] || return 1
  grep -q "img.shields.io/badge/version-${expected}-blue" "$root/README.md" || return 1
  [[ "$(jq -r '.version' "$root/frontend/package.json")" == "$expected" ]] || return 1
  local lock_root lock_pkg
  lock_root="$(jq -r '.version' "$root/frontend/package-lock.json")"
  lock_pkg="$(jq -r '.packages[""].version' "$root/frontend/package-lock.json")"
  [[ "$lock_root" == "$expected" ]] || return 1
  [[ "$lock_pkg" == "$expected" ]] || return 1
}

# bump_version_files <repo_root> <old> <new>
# Rewrites all four sync points from old → new. Uses jq for structural
# updates on package.json / package-lock.json (only root .version and
# packages[""].version are touched — transitive dep versions never).
# Caller is responsible for pre-flight verify_version_sync at <old> and
# post-flight verify at <new>.
bump_version_files() {
  local root="$1" old="$2" new="$3"
  printf "%s\n" "$new" > "$root/VERSION"
  perl -i -pe "s|img\\.shields\\.io/badge/version-\\Q${old}\\E-blue|img.shields.io/badge/version-${new}-blue|g" \
    "$root/README.md"
  local tmp
  tmp="$(mktemp)"
  jq --arg new "$new" '.version = $new' "$root/frontend/package.json" > "$tmp"
  mv "$tmp" "$root/frontend/package.json"
  tmp="$(mktemp)"
  jq --arg new "$new" '.version = $new | .packages[""].version = $new' \
    "$root/frontend/package-lock.json" > "$tmp"
  mv "$tmp" "$root/frontend/package-lock.json"
}

# ---------- orchestration ----------

usage() {
  cat >&2 <<EOF
Usage: $(basename "$0") X.Y.Z [--dry-run]

  Bumps version across all sync points, commits, tags v<X.Y.Z>, pushes,
  opens \$EDITOR for release notes (seeded from git log since last tag),
  then creates a GitHub release.

  --dry-run   Print the planned actions without modifying anything.
EOF
}

require_clean_tree() {
  local root="$1"
  if [[ -n "$(git -C "$root" status --porcelain)" ]]; then
    echo "ERROR: working tree not clean. Commit or stash first." >&2
    git -C "$root" status --short >&2
    exit 5
  fi
}

require_branch() {
  local root="$1" want="$2"
  local cur
  cur="$(git -C "$root" rev-parse --abbrev-ref HEAD)"
  if [[ "$cur" != "$want" ]]; then
    echo "ERROR: must be on branch '$want', currently on '$cur'." >&2
    exit 6
  fi
}

require_in_sync_with_origin() {
  local root="$1" branch="$2"
  git -C "$root" fetch origin "$branch" --quiet
  local local_head remote_head
  local_head="$(git -C "$root" rev-parse HEAD)"
  remote_head="$(git -C "$root" rev-parse "origin/$branch")"
  if [[ "$local_head" != "$remote_head" ]]; then
    echo "ERROR: local '$branch' is not in sync with origin/$branch." >&2
    echo "  local : $local_head" >&2
    echo "  origin: $remote_head" >&2
    exit 7
  fi
}

require_tag_absent() {
  local root="$1" tag="$2"
  if git -C "$root" rev-parse "$tag" >/dev/null 2>&1; then
    echo "ERROR: tag '$tag' already exists locally — delete it first or pick a new version." >&2
    exit 8
  fi
  git -C "$root" fetch origin --tags --quiet
  if git -C "$root" ls-remote --tags origin "refs/tags/$tag" | grep -q .; then
    echo "ERROR: tag '$tag' already exists on origin — release was already published, or partial." >&2
    exit 8
  fi
}

require_gh_authenticated() {
  if ! command -v gh >/dev/null 2>&1; then
    echo "ERROR: 'gh' CLI not found. Install from https://cli.github.com/." >&2
    exit 9
  fi
  if ! gh auth status >/dev/null 2>&1; then
    echo "ERROR: 'gh' not authenticated. Run: gh auth login" >&2
    exit 9
  fi
}

require_jq() {
  if ! command -v jq >/dev/null 2>&1; then
    echo "ERROR: 'jq' not found. Install via brew (macOS) or apt (linux)." >&2
    exit 10
  fi
}

main() {
  set -e
  local new_version="${1:-}"
  if [[ -z "$new_version" ]]; then
    usage
    exit 2
  fi
  shift
  local dry_run="false"
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --dry-run) dry_run="true" ;;
      *) echo "unknown flag: $1" >&2; usage; exit 2 ;;
    esac
    shift
  done

  if ! validate_semver "$new_version"; then
    echo "ERROR: invalid semver '$new_version' (want X.Y.Z, no v prefix, no suffix)." >&2
    usage
    exit 2
  fi

  require_jq
  require_gh_authenticated

  local root
  root="$(git rev-parse --show-toplevel)"

  require_clean_tree "$root"
  require_branch "$root" "main"
  require_in_sync_with_origin "$root" "main"
  require_tag_absent "$root" "v$new_version"

  local current_version
  current_version="$(<"$root/VERSION")"

  if [[ "$current_version" == "$new_version" ]]; then
    echo "ERROR: VERSION already at $new_version — nothing to do." >&2
    exit 1
  fi

  if ! verify_version_sync "$root" "$current_version"; then
    echo "ERROR: version files out of sync with VERSION=$current_version." >&2
    echo "  Expected all of VERSION, README badge, frontend/package.json (.version)," >&2
    echo "  frontend/package-lock.json (.version + .packages[\"\"].version) to be at $current_version." >&2
    exit 3
  fi

  echo "current version: $current_version"
  echo "new version:     $new_version"

  if [[ "$dry_run" == "true" ]]; then
    echo "[dry-run] would update VERSION, README badge, frontend/package.json, frontend/package-lock.json."
    echo "[dry-run] would commit \"chore: bump to v$new_version\", tag v$new_version, push to origin."
    echo "[dry-run] would open \$EDITOR for release notes, then 'gh release create'."
    exit 0
  fi

  bump_version_files "$root" "$current_version" "$new_version"

  if ! verify_version_sync "$root" "$new_version"; then
    echo "ERROR: post-bump sync verification failed; reverting." >&2
    git -C "$root" checkout -- VERSION README.md frontend/package.json frontend/package-lock.json
    exit 4
  fi

  git -C "$root" add VERSION README.md frontend/package.json frontend/package-lock.json
  git -C "$root" commit -m "chore: bump to v$new_version"
  git -C "$root" tag -a "v$new_version" -m "v$new_version"

  # Once we start pushing to origin, partial failure leaves state on remote.
  # Each error message names exactly what's stuck so a human can finish by hand.
  if ! git -C "$root" push origin main; then
    echo "ERROR: failed to push main. Bump commit is local-only; retry: git push origin main" >&2
    echo "       then re-run: bin/release.sh $new_version (it will resume from tag push)." >&2
    exit 11
  fi
  if ! git -C "$root" push origin "v$new_version"; then
    echo "ERROR: failed to push tag v$new_version. main is pushed but tag is local-only." >&2
    echo "       Recover: git push origin v$new_version" >&2
    echo "       Then run: gh release create v$new_version --title v$new_version --generate-notes" >&2
    exit 12
  fi

  local notes_file
  notes_file="$(mktemp -t "release-notes-v${new_version}.XXXXXX").md"
  trap 'rm -f "$notes_file"' EXIT
  local last_tag
  last_tag="$(git -C "$root" describe --tags --abbrev=0 "v${new_version}^" 2>/dev/null || true)"
  {
    echo "## v$new_version"
    echo
    if [[ -n "$last_tag" ]]; then
      echo "### Changes since $last_tag"
      echo
      git -C "$root" log "${last_tag}..v${new_version}" --pretty=format:'- %s' --no-merges
    else
      echo "### Initial release"
      echo
      git -C "$root" log "v${new_version}" --pretty=format:'- %s' --no-merges
    fi
    echo
  } > "$notes_file"

  "${EDITOR:-vi}" "$notes_file"

  if ! gh release create "v$new_version" --title "v$new_version" --notes-file "$notes_file"; then
    echo "ERROR: gh release create failed. Tag v$new_version is published; finish manually:" >&2
    echo "       gh release create v$new_version --title v$new_version --notes-file $notes_file" >&2
    trap - EXIT
    exit 13
  fi

  echo "Released v$new_version."
}

# When sourced for testing, do not run main. Pure functions are defined above.
if [[ "${RELEASE_SH_NO_MAIN:-0}" == "1" ]]; then
  return 0
fi

main "$@"
