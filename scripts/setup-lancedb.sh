#!/usr/bin/env bash
set -euo pipefail

VERSION="${LANCEDB_GO_VERSION:-v0.1.2}"
RUNTIME_HOME="${ECHO_FADE_MEMORY_HOME:-$HOME/.echo-fade-memory}"
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

download() {
  local url="$1"
  local target="$2"
  local tmp="${target}.tmp"
  echo "==> downloading $(basename "$target")"
  curl -L --fail -o "${tmp}" "${url}"
  mv "${tmp}" "${target}"
}

download_if_needed() {
  local url="$1"
  local target="$2"
  if [[ -f "${target}" && "${FORCE}" -eq 0 ]]; then
    echo "==> reuse $(basename "$target")"
    return
  fi
  download "${url}" "${target}"
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
header_url="https://github.com/lancedb/lancedb-go/releases/download/${VERSION}/lancedb.h"
download_if_needed "${header_url}" "${header_target}"

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

if [[ "${archive_fallback}" -eq 0 ]]; then
  lib_target="${lib_dir}/${asset_name}"
  lib_url="https://github.com/lancedb/lancedb-go/releases/download/${VERSION}/${asset_name}"
  download_if_needed "${lib_url}" "${lib_target}"
else
  archive="${lib_dir}/lancedb-go-native-binaries.tar.gz"
  archive_url="https://github.com/lancedb/lancedb-go/releases/download/${VERSION}/lancedb-go-native-binaries.tar.gz"
  if [[ ! -f "${archive}" || "${FORCE}" -eq 1 ]]; then
    download "${archive_url}" "${archive}"
  fi
  extract_from_archive "${archive}" "include/lancedb.h" "${header_target}"
  extract_from_archive "${archive}" "lib/${platform_arch}/liblancedb_go.a" "${lib_dir}/liblancedb_go.a"
  if [[ "${platform}" == "windows" ]]; then
    echo "==> note: windows fallback extracted the static library from the release archive"
  fi
  lib_target="${lib_dir}/liblancedb_go.a"
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
