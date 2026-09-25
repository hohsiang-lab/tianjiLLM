#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
candidate_file="${repo_root}/.codex-client-version"
reviewed_file="${repo_root}/.codex-client-version-reviewed"

read_version_file() {
  local file="$1"
  local label="$2"
  local version

  if [[ ! -f "${file}" ]]; then
    echo "error: missing ${label} file: ${file}" >&2
    return 1
  fi

  version="$(tr -d '[:space:]' <"${file}")"
  if [[ ! "${version}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "error: ${label} must contain one stable semantic version, got '${version}'" >&2
    return 1
  fi

  printf '%s\n' "${version}"
}

candidate_version() {
  read_version_file "${candidate_file}" "Codex client version"
}

reviewed_version() {
  read_version_file "${reviewed_file}" "reviewed Codex client version"
}

latest_stable_version() {
  local release_tag="${1:-}"
  if [[ -z "${release_tag}" ]]; then
    if ! command -v curl >/dev/null 2>&1 || ! command -v jq >/dev/null 2>&1; then
      echo "error: curl and jq are required to fetch the latest Codex release" >&2
      return 1
    fi

    local -a headers=(
      -H "Accept: application/vnd.github+json"
      -H "X-GitHub-Api-Version: 2022-11-28"
      -H "User-Agent: tianjillm-codex-version-check"
    )
    local token="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
    if [[ -n "${token}" ]]; then
      headers+=(-H "Authorization: Bearer ${token}")
    fi

    release_tag="$(
      curl --fail --silent --show-error --location \
        "${headers[@]}" \
        "https://api.github.com/repos/openai/codex/releases/latest" |
        jq --exit-status --raw-output '.tag_name'
    )"
  fi

  if [[ ! "${release_tag}" =~ ^rust-v([0-9]+\.[0-9]+\.[0-9]+)$ ]]; then
    echo "error: latest Codex release tag is not a stable rust-vX.Y.Z tag: '${release_tag}'" >&2
    return 1
  fi

  printf '%s\n' "${BASH_REMATCH[1]}"
}

is_newer_version() {
  local candidate="${1:-}"
  local baseline="${2:-}"
  local candidate_major candidate_minor candidate_patch
  local baseline_major baseline_minor baseline_patch

  if [[ ! "${candidate}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "error: candidate version must be stable semantic version, got '${candidate}'" >&2
    return 2
  fi
  if [[ ! "${baseline}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "error: baseline version must be stable semantic version, got '${baseline}'" >&2
    return 2
  fi

  IFS=. read -r candidate_major candidate_minor candidate_patch <<<"${candidate}"
  IFS=. read -r baseline_major baseline_minor baseline_patch <<<"${baseline}"

  if ((10#${candidate_major} != 10#${baseline_major})); then
    ((10#${candidate_major} > 10#${baseline_major}))
    return
  fi
  if ((10#${candidate_minor} != 10#${baseline_minor})); then
    ((10#${candidate_minor} > 10#${baseline_minor}))
    return
  fi
  ((10#${candidate_patch} > 10#${baseline_patch}))
}

review_status() {
  local candidate
  local reviewed
  candidate="$(candidate_version)"
  reviewed="$(reviewed_version)"

  if [[ "${candidate}" != "${reviewed}" ]]; then
    cat >&2 <<EOF
error: Codex client version ${candidate} has not completed compatibility review.
reviewed version: ${reviewed}

Review upstream changes and Tianji's Codex catalog, Responses HTTP/WebSocket,
compact, image, and header contracts. After migration review and compatibility
tests pass, update .codex-client-version-reviewed to ${candidate}.
EOF
    return 1
  fi

  printf '%s\n' "${candidate}"
}

usage() {
  cat <<'EOF'
Usage:
  scripts/codex-client-version.sh candidate
  scripts/codex-client-version.sh reviewed
  scripts/codex-client-version.sh latest [rust-vX.Y.Z]
  scripts/codex-client-version.sh is-newer CANDIDATE BASELINE
  scripts/codex-client-version.sh review-status
EOF
}

case "${1:-}" in
candidate)
  candidate_version
  ;;
reviewed)
  reviewed_version
  ;;
latest)
  latest_stable_version "${2:-}"
  ;;
is-newer)
  is_newer_version "${2:-}" "${3:-}"
  ;;
review-status)
  review_status
  ;;
*)
  usage >&2
  exit 2
  ;;
esac
