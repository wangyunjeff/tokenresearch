#!/usr/bin/env bash
set -Eeuo pipefail

# One-shot Sub2API binary switcher for a direct-listen deployment.
# Limitation: when Sub2API owns the public port directly, there will still be a
# brief cutover window between old-process exit and new-process bind. The script
# minimizes that window and performs automatic rollback if the new process does
# not become healthy in time.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="${BACKEND_DIR:-$ROOT_DIR/backend}"
CONFIG_FILE="${CONFIG_FILE:-$BACKEND_DIR/config.yaml}"
STATE_DIR="${STATE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/service_codex}"
BACKUP_DIR="${BACKUP_DIR:-$ROOT_DIR/backups/sub2api-bin}"
LOCK_FILE="${LOCK_FILE:-$STATE_DIR/sub2api-switch.lock}"

RELEASE_REPO="${RELEASE_REPO:-Wei-Shaw/sub2api}"
TARGET_VERSION="${TARGET_VERSION:-0.1.115}"
SERVICE_PORT="${SERVICE_PORT:-8080}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:${SERVICE_PORT}/health}"
DOWNLOAD_PROXY="${DOWNLOAD_PROXY:-}"
DOWNLOAD_TIMEOUT="${DOWNLOAD_TIMEOUT:-300}"
STOP_TIMEOUT="${STOP_TIMEOUT:-20}"
START_TIMEOUT="${START_TIMEOUT:-30}"
CURL_RETRY="${CURL_RETRY:-3}"

TMP_DIR=""
STAGED_BINARY=""
BACKUP_PATH=""
ROLLED_BACK=0
SWITCH_STARTED=0

usage() {
  cat <<'EOF'
Usage:
  tools/switch_sub2api_release.sh [options]

Options:
  --version <x.y.z|vx.y.z>  Target version, default: 0.1.115
  --proxy <url>             Download proxy URL, e.g. http://127.0.0.1:7890
  --port <port>             Service port, default: 8080
  --health-url <url>        Health check URL
  --backend-dir <path>      Backend directory, default: <repo>/backend
  --config <path>           Config path, default: <backend>/config.yaml
  --state-dir <path>        State directory, default: ~/.local/state/service_codex
  --backup-dir <path>       Backup directory, default: <repo>/backups/sub2api-bin
  --repo <owner/name>       GitHub repo, default: Wei-Shaw/sub2api
  --help                    Show this message

Examples:
  tools/switch_sub2api_release.sh --version 0.1.115
  tools/switch_sub2api_release.sh --version v0.1.115 --proxy http://127.0.0.1:7890
EOF
}

log() {
  printf '[%s] %s\n' "$(date '+%F %T')" "$*"
}

die() {
  log "ERROR: $*"
  exit 1
}

cleanup_tmp() {
  if [[ -n "$TMP_DIR" && -d "$TMP_DIR" ]]; then
    rm -rf "$TMP_DIR"
  fi
}

extract_config_proxy() {
  [[ -f "$CONFIG_FILE" ]] || return 0
  awk '
    /^[[:space:]]*update:[[:space:]]*$/ {in_update=1; next}
    in_update && /^[^[:space:]]/ {in_update=0}
    in_update && /^[[:space:]]*proxy_url:[[:space:]]*/ {
      sub(/^[[:space:]]*proxy_url:[[:space:]]*/, "", $0)
      gsub(/^"/, "", $0)
      gsub(/"$/, "", $0)
      gsub(/[[:space:]]+#.*$/, "", $0)
      print
      exit
    }
  ' "$CONFIG_FILE"
}

version_number() {
  local raw="$1"
  raw="${raw#v}"
  printf '%s\n' "$raw"
}

version_tag() {
  local raw="$1"
  raw="${raw#v}"
  printf 'v%s\n' "$raw"
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
  local version_num="$1"
  local log_path pid_path current_log current_pid
  mkdir -p "$STATE_DIR"
  log_path="$STATE_DIR/sub2api-v${version_num}.stdout.log"
  pid_path="$STATE_DIR/sub2api-v${version_num}.pid"
  current_log="$STATE_DIR/sub2api-current.stdout.log"
  current_pid="$STATE_DIR/sub2api-current.pid"

  (
    cd "$BACKEND_DIR"
    nohup ./sub2api >"$log_path" 2>&1 < /dev/null &
    echo "$!" >"$pid_path"
  )

  ln -sfn "$log_path" "$current_log"
  ln -sfn "$pid_path" "$current_pid"
}

download_release() {
  local tag="$1"
  local version_num="$2"
  local os arch archive base_url checksum_url expected actual

  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in
    x86_64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) die "unsupported arch: $arch" ;;
  esac

  archive="sub2api_${version_num}_${os}_${arch}.tar.gz"
  base_url="https://github.com/${RELEASE_REPO}/releases/download/${tag}"
  checksum_url="${base_url}/checksums.txt"
  TMP_DIR="$(mktemp -d)"

  local curl_opts=(
    -fsSL
    --connect-timeout 15
    --max-time "$DOWNLOAD_TIMEOUT"
    --retry "$CURL_RETRY"
    --retry-delay 2
    --retry-all-errors
  )
  if [[ -n "$DOWNLOAD_PROXY" ]]; then
    curl_opts+=(--proxy "$DOWNLOAD_PROXY")
  fi

  log "Downloading ${archive}"
  curl "${curl_opts[@]}" "${base_url}/${archive}" -o "${TMP_DIR}/${archive}"
  curl "${curl_opts[@]}" "${checksum_url}" -o "${TMP_DIR}/checksums.txt"

  expected="$(awk -v file="$archive" '$2 == file {print $1}' "${TMP_DIR}/checksums.txt")"
  [[ -n "$expected" ]] || die "checksum entry not found for ${archive}"
  actual="$(sha256sum "${TMP_DIR}/${archive}" | awk '{print $1}')"
  [[ "$expected" == "$actual" ]] || die "checksum mismatch for ${archive}"

  tar -xzf "${TMP_DIR}/${archive}" -C "$TMP_DIR"
  [[ -x "${TMP_DIR}/sub2api" ]] || die "release archive did not contain executable sub2api"
  STAGED_BINARY="${TMP_DIR}/sub2api"
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
    install -m 0755 "$BACKUP_PATH" "$BACKEND_DIR/sub2api"
  fi

  start_process "$ORIGINAL_VERSION"
  if wait_for_health "$START_TIMEOUT"; then
    log "Rollback succeeded; service restored to ${ORIGINAL_VERSION}"
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
  cleanup_tmp
  exit "$exit_code"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --version)
        TARGET_VERSION="$2"
        shift 2
        ;;
      --proxy)
        DOWNLOAD_PROXY="$2"
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
      --backend-dir)
        BACKEND_DIR="$2"
        shift 2
        ;;
      --config)
        CONFIG_FILE="$2"
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
      --repo)
        RELEASE_REPO="$2"
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
  trap cleanup_tmp EXIT

  require_cmd curl
  require_cmd sha256sum
  require_cmd tar
  require_cmd ss
  require_cmd flock
  require_cmd install

  mkdir -p "$STATE_DIR" "$BACKUP_DIR"
  exec 9>"$LOCK_FILE"
  flock -n 9 || die "another switch is already running"

  if [[ -z "$DOWNLOAD_PROXY" ]]; then
    DOWNLOAD_PROXY="$(extract_config_proxy || true)"
  fi

  local version_num tag current_pid staged_version current_version post_pid post_version ts
  version_num="$(version_number "$TARGET_VERSION")"
  tag="$(version_tag "$TARGET_VERSION")"

  log "Target version: ${version_num}"
  if [[ -n "$DOWNLOAD_PROXY" ]]; then
    log "Download proxy: ${DOWNLOAD_PROXY}"
  else
    log "Download proxy: direct"
  fi

  current_pid="$(current_listener_pid || true)"
  [[ -n "$current_pid" ]] || die "no running Sub2API listener found on port ${SERVICE_PORT}"
  ORIGINAL_VERSION="$(binary_version "/proc/${current_pid}/exe")"
  [[ -n "$ORIGINAL_VERSION" ]] || die "failed to detect running version from pid=${current_pid}"
  log "Running pid=${current_pid}, version=${ORIGINAL_VERSION}"

  if [[ "$ORIGINAL_VERSION" == "$version_num" ]]; then
    log "Service already runs ${version_num}; nothing to do"
    return 0
  fi

  download_release "$tag" "$version_num"
  staged_version="$(binary_version "$STAGED_BINARY")"
  [[ "$staged_version" == "$version_num" ]] || die "downloaded binary version mismatch: ${staged_version}"
  log "Downloaded binary verified: ${staged_version}"

  ts="$(date '+%Y%m%d-%H%M%S')"
  BACKUP_PATH="${BACKUP_DIR}/sub2api-${ORIGINAL_VERSION}-${ts}"
  install -m 0755 "$BACKEND_DIR/sub2api" "$BACKUP_PATH"
  log "Backed up current binary to ${BACKUP_PATH}"

  install -m 0755 "$STAGED_BINARY" "$BACKEND_DIR/sub2api"
  current_version="$(binary_version "$BACKEND_DIR/sub2api")"
  [[ "$current_version" == "$version_num" ]] || die "installed binary version mismatch: ${current_version}"
  log "New binary staged at ${BACKEND_DIR}/sub2api"

  SWITCH_STARTED=1
  stop_process "$current_pid"
  cleanup_old_launcher_parent "$current_pid"
  start_process "$version_num"

  if ! wait_for_health "$START_TIMEOUT"; then
    log "New version did not become healthy in ${START_TIMEOUT}s"
    rollback "$(current_listener_pid || true)"
    return 1
  fi

  post_pid="$(current_listener_pid || true)"
  [[ -n "$post_pid" ]] || die "new process started but no listener found on port ${SERVICE_PORT}"
  post_version="$(binary_version "/proc/${post_pid}/exe")"
  [[ "$post_version" == "$version_num" ]] || die "listener pid=${post_pid} is not target version: ${post_version}"

  log "Switch complete: pid=${post_pid}, version=${post_version}, health=${HEALTH_URL}"
  SWITCH_STARTED=0
}

main "$@"
