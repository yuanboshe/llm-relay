#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
script="$repo_root/scripts/install.sh"

fail() {
  echo "test-install-sh: $*" >&2
  exit 1
}

assert_contains() {
  file=$1
  needle=$2
  if ! grep -Fq -- "$needle" "$file"; then
    echo "expected to find: $needle" >&2
    echo "in file: $file" >&2
    echo "--- file contents ---" >&2
    cat "$file" >&2
    fail "assertion failed"
  fi
}

test_local_darwin_signs_before_install() {
  tmp=${TMPDIR:-/tmp}/llmrelay-install-test-$$
  mkdir -p "$tmp/bin" "$tmp/home"
  trap 'rm -rf "$tmp"' EXIT HUP INT TERM

  log="$tmp/log"
  local_bin="$tmp/llmrelay-darwin-arm64"
  cat >"$local_bin" <<'BIN'
#!/usr/bin/env sh
echo "$0 $*" >>"$LLMRELAY_TEST_LOG"
if [ "$1" = "install" ]; then
  exit 0
fi
exit 2
BIN
  chmod +x "$local_bin"

  cat >"$tmp/bin/uname" <<'BIN'
#!/usr/bin/env sh
if [ "$1" = "-s" ]; then
  echo Darwin
elif [ "$1" = "-m" ]; then
  echo arm64
else
  exit 1
fi
BIN
  cat >"$tmp/bin/xattr" <<'BIN'
#!/usr/bin/env sh
echo "xattr $*" >>"$LLMRELAY_TEST_LOG"
BIN
  cat >"$tmp/bin/codesign" <<'BIN'
#!/usr/bin/env sh
echo "codesign $*" >>"$LLMRELAY_TEST_LOG"
BIN
  chmod +x "$tmp/bin/uname" "$tmp/bin/xattr" "$tmp/bin/codesign"

  PATH="$tmp/bin:$PATH" \
    HOME="$tmp/home" \
    LLMRELAY_TEST_LOG="$log" \
    sh "$script" --local "$local_bin" --skip-shell-init --skip-completion >/dev/null

  assert_contains "$log" "xattr -cr $local_bin"
  assert_contains "$log" "codesign --force --sign - $local_bin"
  assert_contains "$log" "$local_bin install --skip-shell-init --skip-completion"
}

test_local_missing_file_fails() {
  tmp=${TMPDIR:-/tmp}/llmrelay-install-test-missing-$$
  mkdir -p "$tmp/bin"
  trap 'rm -rf "$tmp"' EXIT HUP INT TERM

  cat >"$tmp/bin/uname" <<'BIN'
#!/usr/bin/env sh
if [ "$1" = "-s" ]; then
  echo Darwin
else
  echo arm64
fi
BIN
  chmod +x "$tmp/bin/uname"

  if PATH="$tmp/bin:$PATH" sh "$script" --local "$tmp/missing" >/dev/null 2>"$tmp/err"; then
    fail "missing local binary should fail"
  fi
  assert_contains "$tmp/err" "local binary not found"
}

test_local_darwin_signs_before_install
test_local_missing_file_fails

echo "test-install-sh: ok"
