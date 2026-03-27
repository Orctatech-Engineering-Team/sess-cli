#!/usr/bin/env bash
set -euo pipefail

REPO="${SESS_REPO:-Orctatech-Engineering-Team/Sess}"
BIN_NAME="sess"
INSTALL_DIR="${SESS_INSTALL_DIR:-/usr/local/bin}"
VERSION="${SESS_VERSION:-latest}"
BASE_URL="${SESS_BASE_URL:-}"
SKIP_CHECKSUM="${SESS_SKIP_CHECKSUM:-0}"

log() {
  printf '%s\n' "$*"
}

fail() {
  printf 'Error: %s\n' "$*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

detect_os() {
  case "$(uname -s)" in
    Linux) printf 'linux' ;;
    Darwin) printf 'darwin' ;;
    *)
      fail "unsupported operating system: $(uname -s)"
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64' ;;
    aarch64|arm64) printf 'arm64' ;;
    *)
      fail "unsupported architecture: $(uname -m)"
      ;;
  esac
}

checksum_cmd() {
  if command -v sha256sum >/dev/null 2>&1; then
    printf 'sha256sum'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    printf 'shasum -a 256'
    return
  fi
  fail "missing checksum tool: need sha256sum or shasum"
}

ensure_install_dir() {
  if [ -d "$INSTALL_DIR" ] && [ -w "$INSTALL_DIR" ]; then
    return
  fi

  if [ "$(id -u)" -ne 0 ]; then
    fail "install directory $INSTALL_DIR is not writable; rerun with sudo or set SESS_INSTALL_DIR"
  fi

  mkdir -p "$INSTALL_DIR"
}

resolve_base_url() {
  if [ -n "$BASE_URL" ]; then
    printf '%s' "$BASE_URL"
    return
  fi

  if [ "$VERSION" = "latest" ]; then
    printf 'https://github.com/%s/releases/latest/download' "$REPO"
    return
  fi

  printf 'https://github.com/%s/releases/download/%s' "$REPO" "$VERSION"
}

main() {
  need_cmd curl
  need_cmd tar

  local os arch archive_name download_base tmpdir archive_path checksums_path expected_sum actual_sum checksum_tool
  os="$(detect_os)"
  arch="$(detect_arch)"
  archive_name="${BIN_NAME}-${os}-${arch}.tar.gz"
  download_base="$(resolve_base_url)"

  tmpdir="$(mktemp -d)"
  trap "rm -rf '$tmpdir'" EXIT

  archive_path="$tmpdir/$archive_name"
  checksums_path="$tmpdir/checksums.txt"

  log "Downloading $archive_name..."
  curl -fsSL "$download_base/$archive_name" -o "$archive_path" || fail "failed to download $archive_name"

  if [ "$SKIP_CHECKSUM" != "1" ]; then
    log "Downloading checksums.txt..."
    curl -fsSL "$download_base/checksums.txt" -o "$checksums_path" || fail "failed to download checksums.txt"

    expected_sum="$(awk -v name="$archive_name" '$2 == name { print $1 }' "$checksums_path")"
    [ -n "$expected_sum" ] || fail "checksum for $archive_name not found"

    checksum_tool="$(checksum_cmd)"
    actual_sum="$($checksum_tool "$archive_path" | awk '{print $1}')"
    [ "$actual_sum" = "$expected_sum" ] || fail "checksum mismatch for $archive_name"
  fi

  log "Extracting archive..."
  tar -xzf "$archive_path" -C "$tmpdir"
  [ -f "$tmpdir/$BIN_NAME" ] || fail "archive did not contain $BIN_NAME"

  ensure_install_dir

  log "Installing to $INSTALL_DIR/$BIN_NAME..."
  install -m 0755 "$tmpdir/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"

  log "Installed $BIN_NAME to $INSTALL_DIR/$BIN_NAME"
  "$INSTALL_DIR/$BIN_NAME" --version || true
}

main "$@"
