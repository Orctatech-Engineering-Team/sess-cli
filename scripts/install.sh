#!/usr/bin/env bash
set -euo pipefail

REPO="${SESS_REPO:-Orctatech-Engineering-Team/sess-cli}"
BIN_NAME="sess"
INSTALL_DIR="${SESS_INSTALL_DIR:-}"
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

select_install_dir() {
  if [ -z "$INSTALL_DIR" ]; then
    local existing
    existing="$(type -P "$BIN_NAME" || true)"
    if [ -n "$existing" ]; then
      INSTALL_DIR="$(dirname "$existing")"
      log "Found existing installation: $existing"
    else
      [ -n "${HOME:-}" ] || fail "HOME is not set; set SESS_INSTALL_DIR"
      INSTALL_DIR="$HOME/.local/bin"
    fi
  fi
  mkdir -p "$INSTALL_DIR" || fail "cannot create $INSTALL_DIR; choose a writable SESS_INSTALL_DIR"
  INSTALL_DIR="$(cd "$INSTALL_DIR" && pwd -P)"
  [ -w "$INSTALL_DIR" ] || fail "install directory $INSTALL_DIR is not writable; set SESS_INSTALL_DIR=\"\$HOME/.local/bin\" to install for yourself, or explicitly update this directory with administrator permissions"
  if [ -L "$INSTALL_DIR/$BIN_NAME" ]; then
    fail "$INSTALL_DIR/$BIN_NAME is a symlink; update it through its package manager or choose another SESS_INSTALL_DIR"
  fi
  if [ -e "$INSTALL_DIR/$BIN_NAME" ] && [ ! -f "$INSTALL_DIR/$BIN_NAME" ]; then
    fail "$INSTALL_DIR/$BIN_NAME is not a regular file"
  fi
}

report_path() {
  local active
  active="$(type -P "$BIN_NAME" || true)"
  if [ -z "$active" ]; then
    log "Add this directory to PATH in your shell configuration:"
    printf '  export PATH=%q:"$PATH"\n' "$INSTALL_DIR"
  elif [ "$active" -ef "$INSTALL_DIR/$BIN_NAME" ]; then
    log "Run 'hash -r' or open a new terminal, then 'sess --version'."
  else
    log "PATH still selects another installation: $active"
    log "Put $INSTALL_DIR first on PATH to use this copy:"
    printf '  export PATH=%q:"$PATH"\n' "$INSTALL_DIR"
    log "Run 'hash -r' or open a new terminal, then 'command -v sess' and 'sess --version'."
  fi
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
  need_cmd install
  need_cmd mv

  local os arch archive_name download_base archive_path checksums_path expected_sum actual_sum checksum_tool
  select_install_dir
  os="$(detect_os)"
  arch="$(detect_arch)"
  archive_name="${BIN_NAME}-${os}-${arch}.tar.gz"
  download_base="$(resolve_base_url)"

  tmpdir="$(mktemp -d)"
  staged=""
  trap 'rm -rf -- "$tmpdir"; if [ -n "$staged" ]; then rm -f -- "$staged"; fi' EXIT

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
  [ -f "$tmpdir/$BIN_NAME" ] && [ ! -L "$tmpdir/$BIN_NAME" ] || fail "archive did not contain a regular $BIN_NAME binary"

  log "Installing to $INSTALL_DIR/$BIN_NAME..."
  staged="$(mktemp "$INSTALL_DIR/.sess-install.XXXXXX")"
  install -m 0755 "$tmpdir/$BIN_NAME" "$staged"
  mv -f "$staged" "$INSTALL_DIR/$BIN_NAME"
  staged=""

  log "Installed $BIN_NAME to $INSTALL_DIR/$BIN_NAME"
  "$INSTALL_DIR/$BIN_NAME" --version || true
  report_path
}

main "$@"
