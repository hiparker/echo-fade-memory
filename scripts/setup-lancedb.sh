#!/usr/bin/env bash
set -euo pipefail

VERSION="${LANCEDB_GO_VERSION:-v0.1.2}"
RUNTIME_HOME="${ECHO_FADE_MEMORY_HOME:-$HOME/.echo-fade-memory}"
LANCEDB_GO_SOURCE_URL="${LANCEDB_GO_SOURCE_URL:-}"
LANCEDB_RUST_SOURCE_URL="${LANCEDB_RUST_SOURCE_URL:-}"
LANCEDB_ENABLE_SOURCE_MIRROR="${LANCEDB_ENABLE_SOURCE_MIRROR:-1}"
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

if [[ "${STATIC}" -eq 1 ]]; then
  lib_target="${lib_dir}/liblancedb_go.a"
else
  case "${platform}" in
    darwin) lib_target="${lib_dir}/liblancedb_go.dylib" ;;
    linux) lib_target="${lib_dir}/liblancedb_go.so" ;;
    windows) lib_target="${lib_dir}/liblancedb_go.a" ;;
  esac
fi

header_target="${include_dir}/lancedb.h"

if [[ "${FORCE}" -eq 0 && -f "${header_target}" && -f "${lib_target}" ]]; then
  echo "==> reuse $(basename "${header_target}")"
  echo "==> reuse $(basename "${lib_target}")"
else
  if [[ "${platform}" == "windows" ]]; then
    echo "source build is not yet supported on windows" >&2
    exit 1
  fi

  command -v bash >/dev/null
  command -v git >/dev/null
  command -v cargo >/dev/null
  command -v rustup >/dev/null
  command -v cbindgen >/dev/null

  work_dir="$(mktemp -d)"
  trap 'rm -rf "${work_dir}"' EXIT

  candidates=()
  if [[ -n "${LANCEDB_GO_SOURCE_URL}" || -n "${LANCEDB_RUST_SOURCE_URL}" ]]; then
    go_url="${LANCEDB_GO_SOURCE_URL:-https://github.com/lancedb/lancedb-go.git}"
    rust_url="${LANCEDB_RUST_SOURCE_URL:-https://github.com/lancedb/lancedb.git}"
    candidates+=("custom|${go_url}|${rust_url}")
  else
    candidates+=("github|https://github.com/lancedb/lancedb-go.git|https://github.com/lancedb/lancedb.git")
    case "${LANCEDB_ENABLE_SOURCE_MIRROR,,}" in
      0|false|off|no) ;;
      *) candidates+=("gitee mirror|https://gitee.com/hiparker/lancedb-go.git|https://gitee.com/mirrors/lancedb.git") ;;
    esac
  fi

  build_ok=0
  last_err=""
  for candidate in "${candidates[@]}"; do
    IFS='|' read -r label go_url rust_url <<< "${candidate}"
    source_dir="${work_dir}/lancedb-go"
    rm -rf "${source_dir}"

    echo "==> building from source via ${label}" >&2
    echo "==> lancedb-go source: ${go_url}" >&2
    if ! git clone --depth 1 --branch "${VERSION}" "${go_url}" "${source_dir}"; then
      last_err="clone failed from ${go_url}"
      continue
    fi

    if [[ "${rust_url}" != "https://github.com/lancedb/lancedb.git" ]]; then
      cargo_toml="${source_dir}/rust/Cargo.toml"
      sed -i.bak "s#https://github.com/lancedb/lancedb.git#${rust_url}#g" "${cargo_toml}"
      rm -f "${cargo_toml}.bak"
      echo "==> using rust source mirror: ${rust_url}" >&2
    fi

    if ! (cd "${source_dir}" && CARGO_NET_GIT_FETCH_WITH_CLI=true ./scripts/build-native.sh "${platform}" "${normalized_arch}"); then
      last_err="build failed via ${label}"
      continue
    fi

    cp "${source_dir}/include/lancedb.h" "${header_target}"

    if [[ ! -f "${source_dir}/lib/${platform_arch}/$(basename "${lib_target}")" ]]; then
      lib_target="${lib_dir}/liblancedb_go.a"
    fi

    cp "${source_dir}/lib/${platform_arch}/$(basename "${lib_target}")" "${lib_target}"
    build_ok=1
    break
  done

  if [[ "${build_ok}" -ne 1 ]]; then
    echo "${last_err:-source build failed}" >&2
    exit 1
  fi
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
