#!/usr/bin/env bash
# bin/release.sh <bump|publish> X.Y.Z [options]
#
# Two-phase release that NEVER pushes straight to main:
#   1) bump X.Y.Z     — branch + bump all sync points + CHANGELOG, open a PR.
#   2) publish X.Y.Z  — after that PR is merged: tag + push tag + GitHub release.
#
# Sync points bumped/verified together:
#   - /VERSION
#   - /README.md shields.io badge
#   - /frontend/package.json   "version"
#   - /frontend/package-lock.json (root + packages[""].version, 2 occurrences)
#
# Both phases refuse unless all four are in sync at the expected version — this
# catches partial-sync drift. Run `bin/release.sh` with no args for full help.
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
Usage: $(basename "$0") <command> X.Y.Z [options]

Two-phase release that never pushes straight to main.

Commands:
  bump X.Y.Z [-m "changelog line"]
      Prepare the release on a branch and open a PR — NO push to main.
      Creates branch chore/bump-vX.Y.Z, bumps all 4 sync points
      (VERSION, README badge, frontend package.json + package-lock),
      adds a CHANGELOG entry, commits, pushes the branch, opens a PR.
      Review + squash-merge that PR, then run 'publish'.

  publish X.Y.Z [--generate-notes]
      After the bump PR is merged and you are on an up-to-date main:
      tag vX.Y.Z, push the TAG (not main), create the GitHub release.
      Refuses unless VERSION is already X.Y.Z (i.e. the bump PR is merged).
      --generate-notes  Let GitHub auto-generate notes (no \$EDITOR step).

Options:
  --dry-run   Print planned actions without changing anything.

Typical flow:
  $(basename "$0") bump 0.77.0 -m "honest launch outcome (#221)"
  #  ... review + squash-merge the PR on GitHub ...
  git checkout main && git fetch origin && git reset --hard origin/main
  $(basename "$0") publish 0.77.0
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

require_branch_absent() {
  local root="$1" branch="$2"
  if git -C "$root" show-ref --verify --quiet "refs/heads/$branch"; then
    echo "ERROR: branch '$branch' already exists locally — delete it or finish that release first." >&2
    exit 14
  fi
  if git -C "$root" ls-remote --exit-code --heads origin "$branch" >/dev/null 2>&1; then
    echo "ERROR: branch '$branch' already exists on origin — a bump PR is likely already open." >&2
    exit 14
  fi
}

# insert_changelog_entry <repo_root> <version> <note>
# Inserts "## [version] — <today>\n<note>\n" directly above the newest existing
# "## [" entry, keeping the file newest-first. Idempotency is the caller's job
# (the branch-absent + VERSION checks already prevent a double run).
insert_changelog_entry() {
  local root="$1" version="$2" note="$3"
  local file="$root/CHANGELOG.md"
  local today; today="$(date +%Y-%m-%d)"
  local tmp; tmp="$(mktemp)"
  awk -v ver="$version" -v d="$today" -v note="$note" '
    !done && /^## \[/ {
      print "## [" ver "] — " d
      print note
      print ""
      done=1
    }
    { print }
    END { if (!done) { print "## [" ver "] — " d; print note } }
  ' "$file" > "$tmp"
  mv "$tmp" "$file"
}

# ---------- command: bump (PR-based, never pushes to main) ----------

cmd_bump() {
  set -e
  local new_version="${1:-}"
  [[ -n "$new_version" ]] || { usage; exit 2; }
  shift
  local note="" dry_run="false"
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -m|--message) note="${2:-}"; shift ;;
      --dry-run) dry_run="true" ;;
      *) echo "unknown flag: $1" >&2; usage; exit 2 ;;
    esac
    shift
  done

  if ! validate_semver "$new_version"; then
    echo "ERROR: invalid semver '$new_version' (want X.Y.Z, no v prefix, no suffix)." >&2
    usage; exit 2
  fi

  require_jq
  require_gh_authenticated

  local root; root="$(git rev-parse --show-toplevel)"
  local branch="chore/bump-v$new_version"

  require_clean_tree "$root"
  require_branch "$root" "main"
  require_in_sync_with_origin "$root" "main"
  require_tag_absent "$root" "v$new_version"
  require_branch_absent "$root" "$branch"

  local current_version; current_version="$(<"$root/VERSION")"
  if [[ "$current_version" == "$new_version" ]]; then
    echo "ERROR: VERSION already at $new_version — nothing to bump." >&2
    exit 1
  fi
  if ! verify_version_sync "$root" "$current_version"; then
    echo "ERROR: version files out of sync with VERSION=$current_version." >&2
    echo "  Expected VERSION, README badge, frontend/package.json + package-lock all at $current_version." >&2
    exit 3
  fi

  if [[ -z "$note" ]]; then
    note="_TODO: one-line release summary (edit CHANGELOG before merging)_"
    echo "WARNING: no -m note given — inserting a TODO placeholder in CHANGELOG." >&2
  fi

  echo "current version: $current_version"
  echo "new version:     $new_version"
  echo "branch:          $branch"

  if [[ "$dry_run" == "true" ]]; then
    echo "[dry-run] would create branch $branch off main."
    echo "[dry-run] would bump VERSION/README/package.json/package-lock to $new_version."
    echo "[dry-run] would prepend CHANGELOG entry: ## [$new_version] — $(date +%Y-%m-%d) / $note"
    echo "[dry-run] would commit, push $branch, open a PR (base main). No push to main."
    exit 0
  fi

  git -C "$root" checkout -b "$branch"
  bump_version_files "$root" "$current_version" "$new_version"
  if ! verify_version_sync "$root" "$new_version"; then
    echo "ERROR: post-bump sync verification failed; reverting and aborting." >&2
    git -C "$root" checkout -- VERSION README.md frontend/package.json frontend/package-lock.json
    git -C "$root" checkout main
    git -C "$root" branch -D "$branch"
    exit 4
  fi
  insert_changelog_entry "$root" "$new_version" "$note"

  git -C "$root" add VERSION README.md frontend/package.json frontend/package-lock.json CHANGELOG.md
  git -C "$root" commit -m "chore: bump to v$new_version"
  git -C "$root" push -u origin "$branch"

  gh pr create --base main --head "$branch" \
    --title "chore: release v$new_version" \
    --body "Version bump $current_version → $new_version (all 4 sync points) + CHANGELOG.

Changelog line: $note

After squash-merge, publish the release:
  git checkout main && git fetch origin && git reset --hard origin/main
  bin/release.sh publish $new_version"

  echo
  echo "Bump PR opened for v$new_version. Review + squash-merge it, then run:"
  echo "  git checkout main && git fetch origin && git reset --hard origin/main"
  echo "  $(basename "$0") publish $new_version"
}

# ---------- command: publish (tag + release, never pushes to main) ----------

cmd_publish() {
  set -e
  local new_version="${1:-}"
  [[ -n "$new_version" ]] || { usage; exit 2; }
  shift
  local dry_run="false" gen_notes="false"
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --generate-notes) gen_notes="true" ;;
      --dry-run) dry_run="true" ;;
      *) echo "unknown flag: $1" >&2; usage; exit 2 ;;
    esac
    shift
  done

  if ! validate_semver "$new_version"; then
    echo "ERROR: invalid semver '$new_version' (want X.Y.Z, no v prefix, no suffix)." >&2
    usage; exit 2
  fi

  require_jq
  require_gh_authenticated

  local root; root="$(git rev-parse --show-toplevel)"

  require_clean_tree "$root"
  require_branch "$root" "main"
  require_in_sync_with_origin "$root" "main"
  require_tag_absent "$root" "v$new_version"

  local current_version; current_version="$(<"$root/VERSION")"
  if [[ "$current_version" != "$new_version" ]]; then
    echo "ERROR: VERSION is $current_version, not $new_version." >&2
    echo "       Merge the bump PR first:  $(basename "$0") bump $new_version" >&2
    exit 1
  fi
  if ! verify_version_sync "$root" "$new_version"; then
    echo "ERROR: version files out of sync with VERSION=$new_version — the bump merge is incomplete." >&2
    exit 3
  fi

  echo "publishing v$new_version from $(git -C "$root" rev-parse --short HEAD)"

  if [[ "$dry_run" == "true" ]]; then
    echo "[dry-run] would tag v$new_version on main HEAD and push the TAG (not main)."
    echo "[dry-run] would create the GitHub release (notes: $([[ "$gen_notes" == "true" ]] && echo auto || echo \$EDITOR-seeded))."
    exit 0
  fi

  git -C "$root" tag -a "v$new_version" -m "v$new_version"
  if ! git -C "$root" push origin "v$new_version"; then
    echo "ERROR: failed to push tag v$new_version (main is untouched)." >&2
    echo "       Recover: git push origin v$new_version" >&2
    echo "       Then:    gh release create v$new_version --title v$new_version --generate-notes" >&2
    exit 12
  fi

  if [[ "$gen_notes" == "true" ]]; then
    if ! gh release create "v$new_version" --title "v$new_version" --generate-notes; then
      echo "ERROR: gh release create failed. Tag is published; finish with:" >&2
      echo "       gh release create v$new_version --title v$new_version --generate-notes" >&2
      exit 13
    fi
    echo "Released v$new_version."
    return 0
  fi

  local notes_file
  # mktemp + .md rename: keeps editor markdown highlighting without leaking
  # the mktemp-created file (a "$(mktemp ...).md" pattern would leak the
  # original mktemp file as an orphan).
  notes_file="$(mktemp -t release-notes-XXXXXX)"
  mv -- "$notes_file" "${notes_file}.md"
  notes_file="${notes_file}.md"
  # Single-eval the path into the trap body so cleanup survives the function's
  # local going out of scope when it returns (issue #70).
  trap "rm -f -- '$notes_file'" EXIT
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

main() {
  local cmd="${1:-}"
  case "$cmd" in
    bump)    shift; cmd_bump "$@" ;;
    publish) shift; cmd_publish "$@" ;;
    -h|--help|"") usage; exit 2 ;;
    *) echo "unknown command: $cmd" >&2; usage; exit 2 ;;
  esac
}

# When sourced for testing, do not run main. Pure functions are defined above.
if [[ "${RELEASE_SH_NO_MAIN:-0}" == "1" ]]; then
  return 0
fi

main "$@"
