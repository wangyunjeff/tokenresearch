#!/usr/bin/env bash
set -Eeuo pipefail

# Switch a direct-listen Sub2API deployment to a locally built binary.
# This is intended for customized deployments where backend/bin/server should
# replace backend/sub2api with automatic backup, health check, and rollback.
#
# Limitation: because Sub2API itself owns the public port, there is still a
# short cutover window between old-process exit and new-process bind.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="${BACKEND_DIR:-$ROOT_DIR/backend}"
SOURCE_BINARY="${SOURCE_BINARY:-$BACKEND_DIR/bin/server}"
TARGET_BINARY="${TARGET_BINARY:-$BACKEND_DIR/sub2api}"
STATE_DIR="${STATE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/service_codex}"
BACKUP_DIR="${BACKUP_DIR:-$ROOT_DIR/backups/sub2api-bin}"
LOCK_FILE="${LOCK_FILE:-$STATE_DIR/sub2api-switch.lock}"

SERVICE_PORT="${SERVICE_PORT:-8080}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:${SERVICE_PORT}/health}"
STOP_TIMEOUT="${STOP_TIMEOUT:-20}"
START_TIMEOUT="${START_TIMEOUT:-30}"

BACKUP_PATH=""
SOURCE_VERSION=""
TARGET_VERSION=""
TARGET_SHA=""
SOURCE_SHA=""
ROLLED_BACK=0
SWITCH_STARTED=0

usage() {
  cat <<'EOF'
Usage:
  tools/switch_local_sub2api_build.sh [options]

Options:
  --source <path>          Source binary, default: <repo>/backend/bin/server
  --target <path>          Installed runtime binary, default: <repo>/backend/sub2api
  --backend-dir <path>     Backend directory, default: <repo>/backend
  --port <port>            Service port, default: 8080
  --health-url <url>       Health check URL
  --state-dir <path>       State directory, default: ~/.local/state/service_codex
  --backup-dir <path>      Backup directory, default: <repo>/backups/sub2api-bin
  --help                   Show this message

Examples:
  tools/switch_local_sub2api_build.sh
  tools/switch_local_sub2api_build.sh --source backend/bin/server
EOF
}

log() {
  printf '[%s] %s\n' "$(date '+%F %T')" "$*"
}

die() {
  log "ERROR: $*"
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

binary_version() {
  local path="$1"
  "$path" --version 2>&1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1
}

current_listener_pid() {
  ss -ltnp 2>/dev/null | awk -v port=":""$SERVICE_PORT" '
    index($4, port) {
      if (match($0, /pid=[0-9]+/)) {
        pid = substr($0, RSTART + 4, RLENGTH - 4)
        print pid
        exit
      }
    }
  '
}

wait_for_health() {
  local timeout="$1"
  local start_ts now
  start_ts="$(date +%s)"
  while true; do
    if curl -fsS --max-time 2 "$HEALTH_URL" >/dev/null 2>&1; then
      return 0
    fi
    now="$(date +%s)"
    if (( now - start_ts >= timeout )); then
      return 1
    fi
    sleep 0.5
  done
}

wait_for_pid_exit() {
  local pid="$1"
  local timeout="$2"
  local start_ts now
  start_ts="$(date +%s)"
  while kill -0 "$pid" 2>/dev/null; do
    now="$(date +%s)"
    if (( now - start_ts >= timeout )); then
      return 1
    fi
    sleep 0.2
  done
  return 0
}

stop_process() {
  local pid="$1"
  [[ -n "$pid" ]] || return 0
  if ! kill -0 "$pid" 2>/dev/null; then
    return 0
  fi
  log "Stopping current Sub2API process pid=$pid"
  kill -TERM "$pid" 2>/dev/null || true
  if wait_for_pid_exit "$pid" "$STOP_TIMEOUT"; then
    return 0
  fi
  log "Process pid=$pid did not exit in ${STOP_TIMEOUT}s, forcing kill"
  kill -KILL "$pid" 2>/dev/null || true
  wait_for_pid_exit "$pid" 5 || die "failed to stop pid=$pid"
}

cleanup_old_launcher_parent() {
  local pid="$1"
  local ppid
  [[ -n "$pid" ]] || return 0
  ppid="$(ps -o ppid= -p "$pid" 2>/dev/null | tr -d '[:space:]' || true)"
  [[ -n "$ppid" ]] || return 0
  if kill -0 "$ppid" 2>/dev/null; then
    local parent_cmd
    parent_cmd="$(ps -o cmd= -p "$ppid" 2>/dev/null || true)"
    if [[ "$parent_cmd" == *"nohup ./sub2api"* ]]; then
      kill -TERM "$ppid" 2>/dev/null || true
    fi
  fi
}

start_process() {
  local version_label="$1"
  local log_path pid_path current_log current_pid
  mkdir -p "$STATE_DIR"
  log_path="$STATE_DIR/sub2api-v${version_label}.stdout.log"
  pid_path="$STATE_DIR/sub2api-v${version_label}.pid"
  current_log="$STATE_DIR/sub2api-current.stdout.log"
  current_pid="$STATE_DIR/sub2api-current.pid"

  (
    cd "$BACKEND_DIR"
    # Do not leak the lock fd into the long-running service process.
    exec 9>&-
    nohup ./sub2api >"$log_path" 2>&1 < /dev/null &
    echo "$!" >"$pid_path"
  )

  ln -sfn "$log_path" "$current_log"
  ln -sfn "$pid_path" "$current_pid"
}

rollback() {
  local failed_pid="$1"
  [[ "$ROLLED_BACK" -eq 0 ]] || return 0
  ROLLED_BACK=1

  if [[ -n "$failed_pid" ]] && kill -0 "$failed_pid" 2>/dev/null; then
    kill -TERM "$failed_pid" 2>/dev/null || true
    wait_for_pid_exit "$failed_pid" 5 || true
    kill -KILL "$failed_pid" 2>/dev/null || true
  fi

  if [[ -n "$BACKUP_PATH" && -f "$BACKUP_PATH" ]]; then
    log "Rolling back binary from $BACKUP_PATH"
    install -m 0755 "$BACKUP_PATH" "$TARGET_BINARY"
  fi

  start_process "${TARGET_VERSION:-rollback}"
  if wait_for_health "$START_TIMEOUT"; then
    log "Rollback succeeded; service restored to ${TARGET_VERSION:-unknown}"
    return 0
  fi

  die "rollback failed; manual intervention required"
}

on_error() {
  local exit_code="$1"
  local failed_pid=""
  failed_pid="$(current_listener_pid || true)"
  if [[ "$SWITCH_STARTED" -eq 1 ]]; then
    rollback "$failed_pid" || true
  fi
  exit "$exit_code"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --source)
        SOURCE_BINARY="$2"
        shift 2
        ;;
      --target)
        TARGET_BINARY="$2"
        shift 2
        ;;
      --backend-dir)
        BACKEND_DIR="$2"
        shift 2
        ;;
      --port)
        SERVICE_PORT="$2"
        HEALTH_URL="http://127.0.0.1:${SERVICE_PORT}/health"
        shift 2
        ;;
      --health-url)
        HEALTH_URL="$2"
        shift 2
        ;;
      --state-dir)
        STATE_DIR="$2"
        LOCK_FILE="$STATE_DIR/sub2api-switch.lock"
        shift 2
        ;;
      --backup-dir)
        BACKUP_DIR="$2"
        shift 2
        ;;
      --help|-h)
        usage
        exit 0
        ;;
      *)
        die "unknown argument: $1"
        ;;
    esac
  done
}

main() {
  parse_args "$@"
  trap 'on_error $?' ERR

  require_cmd curl
  require_cmd ss
  require_cmd flock
  require_cmd install
  require_cmd sha256sum

  [[ -x "$SOURCE_BINARY" ]] || die "source binary is missing or not executable: $SOURCE_BINARY"
  [[ -x "$TARGET_BINARY" ]] || die "target binary is missing or not executable: $TARGET_BINARY"

  mkdir -p "$STATE_DIR" "$BACKUP_DIR"
  exec 9>"$LOCK_FILE"
  flock -n 9 || die "another switch is already running"

  local current_pid ts source_label post_pid post_version current_installed_version
  current_pid="$(current_listener_pid || true)"
  [[ -n "$current_pid" ]] || die "no running Sub2API listener found on port ${SERVICE_PORT}"

  TARGET_VERSION="$(binary_version "/proc/${current_pid}/exe" || true)"
  current_installed_version="$(binary_version "$TARGET_BINARY" || true)"
  SOURCE_VERSION="$(binary_version "$SOURCE_BINARY" || true)"
  TARGET_SHA="$(sha256sum "$TARGET_BINARY" | awk '{print $1}')"
  SOURCE_SHA="$(sha256sum "$SOURCE_BINARY" | awk '{print $1}')"

  log "Running pid=${current_pid}, live_version=${TARGET_VERSION:-unknown}, installed_version=${current_installed_version:-unknown}"
  log "Source binary: $SOURCE_BINARY"
  log "Target binary: $TARGET_BINARY"
  log "Source version=${SOURCE_VERSION:-unknown}, source_sha=${SOURCE_SHA}"
  log "Target sha=${TARGET_SHA}"

  if [[ "$SOURCE_SHA" == "$TARGET_SHA" ]]; then
    log "Source and target binaries are identical; nothing to do"
    return 0
  fi

  ts="$(date '+%Y%m%d-%H%M%S')"
  BACKUP_PATH="${BACKUP_DIR}/sub2api-localbuild-${TARGET_VERSION:-unknown}-${ts}"
  install -m 0755 "$TARGET_BINARY" "$BACKUP_PATH"
  log "Backed up current binary to ${BACKUP_PATH}"

  install -m 0755 "$SOURCE_BINARY" "$TARGET_BINARY"
  log "Installed local build to ${TARGET_BINARY}"

  SWITCH_STARTED=1
  stop_process "$current_pid"
  cleanup_old_launcher_parent "$current_pid"
  source_label="${SOURCE_VERSION:-localbuild}"
  start_process "$source_label"

  if ! wait_for_health "$START_TIMEOUT"; then
    log "New local build did not become healthy in ${START_TIMEOUT}s"
    rollback "$(current_listener_pid || true)"
    return 1
  fi

  post_pid="$(current_listener_pid || true)"
  [[ -n "$post_pid" ]] || die "new process started but no listener found on port ${SERVICE_PORT}"
  post_version="$(binary_version "/proc/${post_pid}/exe" || true)"

  log "Switch complete: pid=${post_pid}, version=${post_version:-unknown}, health=${HEALTH_URL}"
  SWITCH_STARTED=0
}

main "$@"
