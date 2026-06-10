#!/usr/bin/env sh
set -eu

repo=${LLMRELAY_REPO:-yuanboshe/llm-relay}
version=${LLMRELAY_VERSION:-latest}
base_url=${LLMRELAY_INSTALL_BASE_URL:-}
local_binary=
install_args=

usage() {
  cat <<'EOF'
Install llmrelay for the current user.

Usage:
  scripts/install.sh
  scripts/install.sh --local ./llmrelay-darwin-arm64

Options:
  --local PATH          Install from an existing local binary.
  --version VERSION     Download a specific release tag instead of latest.
  --repo OWNER/REPO     GitHub repository to download from.
  --base-url URL        Download base URL that contains the binary and SHA256SUMS.
  --skip-shell-init     Pass through to llmrelay install.
  --skip-completion     Pass through to llmrelay install.
  -h, --help            Show this help.
EOF
}

log() {
  printf '%s\n' "$*" >&2
}

die() {
  log "error: $*"
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --local)
      [ "$#" -ge 2 ] || die "--local requires a path"
      local_binary=$2
      shift 2
      ;;
    --version)
      [ "$#" -ge 2 ] || die "--version requires a value"
      version=$2
      shift 2
      ;;
    --repo)
      [ "$#" -ge 2 ] || die "--repo requires OWNER/REPO"
      repo=$2
      shift 2
      ;;
    --base-url)
      [ "$#" -ge 2 ] || die "--base-url requires a URL"
      base_url=$2
      shift 2
      ;;
    --skip-shell-init)
      install_args="$install_args --skip-shell-init"
      shift
      ;;
    --skip-completion)
      install_args="$install_args --skip-completion"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown option: $1"
      ;;
  esac
done

detect_os() {
  case "$(uname -s)" in
    Darwin) printf 'darwin' ;;
    Linux) printf 'linux' ;;
    *) die "unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    arm64|aarch64) printf 'arm64' ;;
    x86_64|amd64) printf 'amd64' ;;
    *) die "unsupported CPU architecture: $(uname -m)" ;;
  esac
}

download() {
  url=$1
  output=$2
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$output"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$output"
  else
    die "curl or wget is required for downloads"
  fi
}

sha256_file() {
  file=$1
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
  else
    die "sha256sum or shasum is required to verify downloads"
  fi
}

download_binary() {
  os=$1
  arch=$2
  tmp=$3
  asset="llmrelay-$os-$arch"
  if [ -z "$base_url" ]; then
    if [ "$version" = "latest" ]; then
      base_url="https://github.com/$repo/releases/latest/download"
    else
      base_url="https://github.com/$repo/releases/download/$version"
    fi
  fi
  base_url=${base_url%/}

  binary="$tmp/$asset"
  sums="$tmp/SHA256SUMS"
  log "downloading $asset from $base_url"
  download "$base_url/$asset" "$binary"
  download "$base_url/SHA256SUMS" "$sums"

  expected=$(awk -v name="$asset" '$2 == name {print $1}' "$sums")
  [ -n "$expected" ] || die "SHA256SUMS does not contain $asset"
  actual=$(sha256_file "$binary")
  [ "$actual" = "$expected" ] || die "checksum mismatch for $asset"
  printf '%s\n' "$binary"
}

prepare_binary() {
  binary=$1
  os=$2
  chmod +x "$binary"

  if [ "$os" = "darwin" ]; then
    if command -v xattr >/dev/null 2>&1; then
      xattr -cr "$binary" || true
    fi
    if command -v codesign >/dev/null 2>&1; then
      codesign --force --sign - "$binary" >/dev/null
    else
      log "warning: codesign not found; macOS may reject this binary"
    fi
  fi
}

os=$(detect_os)
arch=$(detect_arch)

tmp=
cleanup() {
  if [ -n "$tmp" ] && [ -d "$tmp" ]; then
    rm -rf "$tmp"
  fi
}
trap cleanup EXIT HUP INT TERM

if [ -n "$local_binary" ]; then
  [ -f "$local_binary" ] || die "local binary not found: $local_binary"
  binary=$local_binary
else
  tmp=$(mktemp -d "${TMPDIR:-/tmp}/llmrelay-install.XXXXXX")
  binary=$(download_binary "$os" "$arch" "$tmp")
fi

prepare_binary "$binary" "$os"

log "running llmrelay install"
# shellcheck disable=SC2086
"$binary" install $install_args

log "verifying installation"
if command -v llmrelay >/dev/null 2>&1; then
  llmrelay version
else
  log "llmrelay installed; open a new shell if ~/.local/bin is not on PATH yet"
fi
