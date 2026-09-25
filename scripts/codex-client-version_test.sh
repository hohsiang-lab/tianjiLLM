#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

mkdir -p "${tmp_dir}/scripts"
cp "${repo_root}/scripts/codex-client-version.sh" "${tmp_dir}/scripts/"
chmod +x "${tmp_dir}/scripts/codex-client-version.sh"

version_script="${tmp_dir}/scripts/codex-client-version.sh"

assert_eq() {
  local expected="$1"
  local actual="$2"
  if [[ "${actual}" != "${expected}" ]]; then
    echo "expected '${expected}', got '${actual}'" >&2
    exit 1
  fi
}

printf '0.144.1\n' >"${tmp_dir}/.codex-client-version"
printf '0.144.1\n' >"${tmp_dir}/.codex-client-version-reviewed"

assert_eq "0.144.1" "$("${version_script}" candidate)"
assert_eq "0.144.1" "$("${version_script}" reviewed)"
assert_eq "0.144.1" "$("${version_script}" latest rust-v0.144.1)"
assert_eq "0.144.1" "$("${version_script}" review-status)"

if ! "${version_script}" is-newer 0.145.0 0.144.1; then
  echo "expected 0.145.0 to be newer than 0.144.1" >&2
  exit 1
fi
if "${version_script}" is-newer 0.144.1 0.145.0; then
  echo "expected version downgrade to be rejected" >&2
  exit 1
fi
if "${version_script}" is-newer 0.144.1 0.144.1; then
  echo "expected equal versions not to be newer" >&2
  exit 1
fi

if "${version_script}" latest rust-v0.145.0-alpha.1 >/dev/null 2>&1; then
  echo "expected prerelease tag to be rejected" >&2
  exit 1
fi

printf '0.145.0\n' >"${tmp_dir}/.codex-client-version"
if "${version_script}" review-status >/dev/null 2>&1; then
  echo "expected unreviewed version change to be rejected" >&2
  exit 1
fi

printf 'not-a-version\n' >"${tmp_dir}/.codex-client-version"
if "${version_script}" candidate >/dev/null 2>&1; then
  echo "expected invalid version file to be rejected" >&2
  exit 1
fi

echo "codex client version script tests passed"
