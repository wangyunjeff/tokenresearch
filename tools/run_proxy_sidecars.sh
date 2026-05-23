#!/usr/bin/env bash
set -euo pipefail

ROOT="${ROOT:-/mnt/data/service_codex2}"
SIDECAR_ROOT="${SIDECAR_ROOT:-$ROOT/backend/data/proxy_sidecars}"
CLASH_BIN="${CLASH_BIN:-/home/ike/Downloads/clash/clash-linux-amd64-v1.18.0}"

declare -a CHILD_PIDS=()
declare -a PID_FILES=()

log() {
  printf '[proxy-sidecars] %s\n' "$*"
}

port_is_listening() {
  local port="$1"
  ss -ltn | awk '{print $4}' | grep -qE "127\\.0\\.0\\.1:${port}$"
}

stop_pid_file() {
  local pid_file="$1"
  local config_path="$2"

  [[ -f "$pid_file" ]] || return 0

  local pid
  pid="$(tr -dc '0-9' < "$pid_file" || true)"
  [[ -n "$pid" ]] || return 0
  kill -0 "$pid" 2>/dev/null || return 0

  local cmd
  cmd="$(ps -p "$pid" -o args= 2>/dev/null || true)"
  if [[ "$cmd" != *"$config_path"* ]]; then
    log "pid $pid from $pid_file is not this sidecar; leaving it alone"
    return 0
  fi

  log "stopping existing sidecar pid=$pid config=$config_path"
  kill "$pid" 2>/dev/null || true

  for _ in {1..50}; do
    kill -0 "$pid" 2>/dev/null || return 0
    sleep 0.1
  done

  log "existing sidecar pid=$pid did not stop cleanly; killing"
  kill -KILL "$pid" 2>/dev/null || true
}

cleanup() {
  local status="${1:-0}"

  trap - TERM INT EXIT

  for pid in "${CHILD_PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done

  for pid in "${CHILD_PIDS[@]:-}"; do
    wait "$pid" 2>/dev/null || true
  done

  for pid_file in "${PID_FILES[@]:-}"; do
    rm -f "$pid_file"
  done

  exit "$status"
}

trap 'cleanup 0' TERM INT
trap 'cleanup $?' EXIT

[[ -x "$CLASH_BIN" ]] || {
  log "missing executable Clash binary: $CLASH_BIN"
  exit 1
}

shopt -s nullglob
sidecar_dirs=("$SIDECAR_ROOT"/*)
shopt -u nullglob

if [[ "${#sidecar_dirs[@]}" -eq 0 ]]; then
  log "no sidecar directories found under $SIDECAR_ROOT"
  exit 1
fi

for sidecar_dir in "${sidecar_dirs[@]}"; do
  config_path="$sidecar_dir/config.yaml"
  [[ -f "$config_path" ]] || continue

  name="$(basename "$sidecar_dir")"
  home_dir="$sidecar_dir/home"
  log_path="$sidecar_dir/clash.log"
  pid_file="$sidecar_dir/clash.pid"
  socks_port="$(awk '/^socks-port:/ {print $2; exit}' "$config_path")"

  if [[ -z "$socks_port" ]]; then
    log "missing socks-port in $config_path"
    exit 1
  fi

  stop_pid_file "$pid_file" "$config_path"

  if port_is_listening "$socks_port"; then
    log "port $socks_port is already listening before starting $name"
    exit 1
  fi

  mkdir -p "$home_dir"
  : > "$log_path"

  "$CLASH_BIN" -d "$home_dir" -f "$config_path" >> "$log_path" 2>&1 &
  pid="$!"
  CHILD_PIDS+=("$pid")
  PID_FILES+=("$pid_file")
  printf '%s\n' "$pid" > "$pid_file"
  log "started $name pid=$pid socks_port=$socks_port"
done

sleep 1

for sidecar_dir in "${sidecar_dirs[@]}"; do
  config_path="$sidecar_dir/config.yaml"
  [[ -f "$config_path" ]] || continue

  name="$(basename "$sidecar_dir")"
  pid_file="$sidecar_dir/clash.pid"
  socks_port="$(awk '/^socks-port:/ {print $2; exit}' "$config_path")"
  pid="$(cat "$pid_file")"

  if ! kill -0 "$pid" 2>/dev/null; then
    log "$name exited during startup"
    exit 1
  fi

  if ! port_is_listening "$socks_port"; then
    log "$name started but socks_port=$socks_port is not listening"
    exit 1
  fi
done

log "all proxy sidecars are running"

wait -n "${CHILD_PIDS[@]}"
status="$?"
log "a sidecar exited with status $status; stopping the group"
cleanup "$status"
