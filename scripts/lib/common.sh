# shellcheck shell=sh
# Helpers shared by the scripts under scripts/. A script sources this
# file first, since sourcing resets PROG and SCRATCH, then sets PROG to
# the name that die's messages start with. make_scratch owns the EXIT,
# INT and TERM traps.
# POSIX sh (ADR 4): it must run under dash, macOS sh and Git Bash.

PROG=''
# Not TMP, TEMP or TMPDIR: Windows exports TMP and TEMP, and macOS
# TMPDIR, so setting one here would change the temporary directory of
# every tool the script runs. Under Git Bash an empty TMP leaves MinGW
# gcc without a usable temporary directory.
SCRATCH=''

# die <message...>: report that the script cannot run, and exit 2.
die() {
  printf '%s: %s\n' "${PROG}" "$*" >&2
  exit 2
}

# cleanup: remove SCRATCH, if make_scratch created it.
cleanup() {
  if [ -n "${SCRATCH}" ]; then rm -rf -- "${SCRATCH}"; fi
}

# on_signal <signal>: clean up, then stop with the signal, so the caller
# sees how the script ended.
on_signal() {
  cleanup
  trap - "$1"
  kill -s "$1" "$$"
}

# make_scratch: set SCRATCH to a new temporary directory, removed when
# the script exits or is stopped by INT or TERM.
make_scratch() {
  trap cleanup EXIT
  trap 'on_signal INT' INT
  trap 'on_signal TERM' TERM
  SCRATCH=$(mktemp -d) || die "cannot create a temporary directory"
}
