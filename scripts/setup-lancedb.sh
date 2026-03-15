#!/usr/bin/env bash
set -euo pipefail

VERSION="${LANCEDB_GO_VERSION:-v0.1.2}"
RUNTIME_HOME="${ECHO_FADE_MEMORY_HOME:-$HOME/.echo-fade-memory}"
LANCEDB_GO_SOURCE_URL="${LANCEDB_GO_SOURCE_URL:-https://github.com/lancedb/lancedb-go.git}"
LANCEDB_RUST_SOURCE_URL="${LANCEDB_RUST_SOURCE_URL:-https://github.com/lancedb/lancedb.git}"
LANCEDB_GO_RELEASE_BASE_URLS="${LANCEDB_GO_RELEASE_BASE_URLS:-}"
LANCEDB_GO_RELEASE_BASE_URL="${LANCEDB_GO_RELEASE_BASE_URL:-}"
FORCE=0
STATIC=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --force)
      FORCE=1
      shift
      ;;
    --static)
      STATIC=1
      shift
      ;;
    *)
      echo "unknown option: $1" >&2
      exit 1
      ;;
  esac
done

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
  Darwin) platform="darwin" ;;
  Linux) platform="linux" ;;
  MINGW*|MSYS*|CYGWIN*) platform="windows" ;;
  *)
    echo "unsupported platform: $os" >&2
    exit 1
    ;;
esac

case "$arch" in
  x86_64|amd64) normalized_arch="amd64" ;;
  arm64|aarch64) normalized_arch="arm64" ;;
  *)
    echo "unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

platform_arch="${platform}_${normalized_arch}"
include_dir="${RUNTIME_HOME}/include"
lib_dir="${RUNTIME_HOME}/lib/${platform_arch}"

mkdir -p "${include_dir}" "${lib_dir}"

release_bases=()
if [[ -n "${LANCEDB_GO_RELEASE_BASE_URLS}" ]]; then
  IFS=',' read -r -a release_bases <<< "${LANCEDB_GO_RELEASE_BASE_URLS}"
elif [[ -n "${LANCEDB_GO_RELEASE_BASE_URL}" ]]; then
  release_bases=("${LANCEDB_GO_RELEASE_BASE_URL}")
else
  release_bases=("https://github.com/lancedb/lancedb-go/releases/download/${VERSION}")
fi

trim_url() {
  local raw="$1"
  raw="${raw#"${raw%%[![:space:]]*}"}"
  raw="${raw%"${raw##*[![:space:]]}"}"
  raw="${raw%/}"
  printf '%s' "${raw}"
}

download() {
  local url="$1"
  local target="$2"
  local tmp="${target}.tmp"
  echo "==> downloading $(basename "$target")"
  curl -L --fail -o "${tmp}" "${url}"
  mv "${tmp}" "${target}"
}

download_from_bases_if_needed() {
  local asset="$1"
  local target="$2"
  if [[ -f "${target}" && "${FORCE}" -eq 0 ]]; then
    echo "==> reuse $(basename "$target")"
    return
  fi
  local base
  local url
  local err=1
  for base in "${release_bases[@]}"; do
    base="$(trim_url "${base}")"
    [[ -z "${base}" ]] && continue
    url="${base}/${asset}"
    echo "==> downloading $(basename "$target") from ${base}"
    if download "${url}" "${target}"; then
      err=0
      break
    fi
  done
  if [[ "${err}" -ne 0 ]]; then
    return 1
  fi
}

extract_from_archive() {
  local archive="$1"
  local member="$2"
  local target="$3"
  local tmp_dir
  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "${tmp_dir}"' EXIT
  tar -xzf "${archive}" -C "${tmp_dir}" "${member}"
  mv "${tmp_dir}/${member}" "${target}"
  trap - EXIT
  rm -rf "${tmp_dir}"
}

header_target="${include_dir}/lancedb.h"

asset_name=""
archive_fallback=0

if [[ "${STATIC}" -eq 1 ]]; then
  asset_name="liblancedb_go.a"
else
  case "${platform}" in
    darwin) asset_name="liblancedb_go.dylib" ;;
    linux) asset_name="liblancedb_go.so" ;;
    windows) archive_fallback=1 ;;
  esac
fi

release_failed=0
if ! download_from_bases_if_needed "lancedb.h" "${header_target}"; then
  release_failed=1
fi

if [[ "${release_failed}" -eq 0 ]]; then
  if [[ "${archive_fallback}" -eq 0 ]]; then
    lib_target="${lib_dir}/${asset_name}"
    if ! download_from_bases_if_needed "${asset_name}" "${lib_target}"; then
      release_failed=1
    fi
  else
    archive="${lib_dir}/lancedb-go-native-binaries.tar.gz"
    if ! download_from_bases_if_needed "lancedb-go-native-binaries.tar.gz" "${archive}"; then
      release_failed=1
    else
      extract_from_archive "${archive}" "include/lancedb.h" "${header_target}"
      extract_from_archive "${archive}" "lib/${platform_arch}/liblancedb_go.a" "${lib_dir}/liblancedb_go.a"
      if [[ "${platform}" == "windows" ]]; then
        echo "==> note: windows fallback extracted the static library from the release archive"
      fi
      lib_target="${lib_dir}/liblancedb_go.a"
    fi
  fi
fi

build_from_source() {
  if [[ "${platform}" == "windows" ]]; then
    echo "source fallback is not yet supported on windows" >&2
    return 1
  fi

  command -v git >/dev/null
  command -v cargo >/dev/null
  command -v rustup >/dev/null
  command -v cbindgen >/dev/null

  local work_dir
  work_dir="$(mktemp -d)"
  trap 'rm -rf "${work_dir}"' EXIT

  local source_dir="${work_dir}/lancedb-go"
  echo "==> falling back to source build" >&2
  git clone --depth 1 --branch "${VERSION}" "${LANCEDB_GO_SOURCE_URL}" "${source_dir}"

  if [[ "${LANCEDB_RUST_SOURCE_URL}" != "https://github.com/lancedb/lancedb.git" ]]; then
    python3 - <<'PY' "${source_dir}/rust/Cargo.toml" "${LANCEDB_RUST_SOURCE_URL}"
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
target = sys.argv[2]
old = "https://github.com/lancedb/lancedb.git"
text = path.read_text()
if old not in text:
    raise SystemExit(f"{old} not found in {path}")
path.write_text(text.replace(old, target))
PY
    echo "==> using rust source mirror: ${LANCEDB_RUST_SOURCE_URL}" >&2
  fi

  (cd "${source_dir}" && CARGO_NET_GIT_FETCH_WITH_CLI=true ./scripts/build-native.sh "${platform}" "${normalized_arch}")
  cp "${source_dir}/include/lancedb.h" "${header_target}"

  local built_name
  if [[ "${STATIC}" -eq 1 ]]; then
    built_name="liblancedb_go.a"
  else
    case "${platform}" in
      darwin) built_name="liblancedb_go.dylib" ;;
      linux) built_name="liblancedb_go.so" ;;
      *) built_name="liblancedb_go.a" ;;
    esac
  fi

  if [[ ! -f "${source_dir}/lib/${platform_arch}/${built_name}" ]]; then
    built_name="liblancedb_go.a"
  fi

  cp "${source_dir}/lib/${platform_arch}/${built_name}" "${lib_dir}/${built_name}"
  lib_target="${lib_dir}/${built_name}"
}

if [[ "${release_failed}" -ne 0 ]]; then
  build_from_source
fi

extra_ldflags=""
case "${platform}" in
  darwin) extra_ldflags="-framework Security -framework CoreFoundation" ;;
  linux) extra_ldflags="-ldl -lm -lpthread" ;;
esac

echo
echo "LanceDB runtime home: ${RUNTIME_HOME}"
echo "Header: ${header_target}"
echo "Library: ${lib_target}"
echo
echo "CGO_CFLAGS=-I${include_dir}"
echo "CGO_LDFLAGS=${lib_target} ${extra_ldflags}"
