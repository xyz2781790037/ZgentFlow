#!/usr/bin/env bash
set -Eeuo pipefail

ZEALRAG_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ZEALRAG_RUNTIME_DIR="${ZEALRAG_RUNTIME_DIR:-$ZEALRAG_ROOT/.runtime}"
ZEALRAG_DATA_DIR="${ZEALRAG_DATA_DIR:-$ZEALRAG_ROOT/.local-data}"
ZEALRAG_LOG_DIR="$ZEALRAG_DATA_DIR/logs"
ZEALRAG_APP_PORT="${ZEALRAG_APP_PORT:-8081}"
ZEALRAG_DB_PORT="${ZEALRAG_DB_PORT:-54329}"
ZEALRAG_REDIS_PORT="${ZEALRAG_REDIS_PORT:-6389}"
ZEALRAG_DOCREADER_PORT="${ZEALRAG_DOCREADER_PORT:-50061}"

mkdir -p "$ZEALRAG_RUNTIME_DIR" "$ZEALRAG_DATA_DIR" "$ZEALRAG_LOG_DIR"

log() {
  printf '[ZgentFlow] %s\n' "$*"
}

fail() {
  printf '[ZgentFlow] error: %s\n' "$*" >&2
  exit 1
}

pid_is_running() {
  local pid="${1:-}"
  local state
  [[ "$pid" =~ ^[0-9]+$ ]] || return 1
  kill -0 "$pid" >/dev/null 2>&1 || return 1
  state="$(ps -o stat= -p "$pid" 2>/dev/null || true)"
  [[ -n "$state" && "${state:0:1}" != "Z" ]]
}

pid_executable_matches() {
  local pid="$1"
  local expected="$2"
  local actual expected_real
  pid_is_running "$pid" || return 1
  [[ -e "$expected" ]] || return 1
  actual="$(readlink "/proc/$pid/exe" 2>/dev/null || true)"
  expected_real="$(readlink -f "$expected" 2>/dev/null || true)"
  actual="${actual% (deleted)}"
  [[ -n "$actual" && "$actual" == "$expected_real" ]]
}

pid_command_contains() {
  local pid="$1"
  local needle="$2"
  local command_line
  pid_is_running "$pid" || return 1
  command_line="$(tr '\0' ' ' <"/proc/$pid/cmdline" 2>/dev/null || true)"
  [[ "$command_line" == *"$needle"* ]]
}

pid_workdir_matches() {
  local pid="$1"
  local expected="$2"
  local actual expected_real
  pid_is_running "$pid" || return 1
  actual="$(readlink -f "/proc/$pid/cwd" 2>/dev/null || true)"
  expected_real="$(readlink -f "$expected" 2>/dev/null || true)"
  [[ -n "$actual" && "$actual" == "$expected_real" ]]
}

port_is_listening() {
  local host="$1"
  local port="$2"
  (echo >"/dev/tcp/$host/$port") >/dev/null 2>&1
}

listener_pid_for_port() {
  local port="$1"
  local output
  command -v ss >/dev/null 2>&1 || return 1
  output="$(ss -H -ltnp "sport = :$port" 2>/dev/null || true)"
  if [[ "$output" =~ pid=([0-9]+) ]]; then
    printf '%s\n' "${BASH_REMATCH[1]}"
    return 0
  fi
  return 1
}

process_summary() {
  local pid="$1"
  local command_line
  command_line="$(tr '\0' ' ' <"/proc/$pid/cmdline" 2>/dev/null || true)"
  command_line="${command_line% }"
  if [[ -n "$command_line" ]]; then
    printf 'pid %s (%s)' "$pid" "$command_line"
  else
    printf 'pid %s' "$pid"
  fi
}

stop_pid_gracefully() {
  local name="$1"
  local pid="$2"
  local attempts="${3:-50}"
  local i
  pid_is_running "$pid" || return 0

  log "stopping $name (pid $pid)"
  kill "$pid" >/dev/null 2>&1 || true
  for ((i = 1; i <= attempts; i++)); do
    pid_is_running "$pid" || return 0
    sleep 0.2
  done

  log "$name did not stop in time; forcing shutdown (pid $pid)"
  kill -KILL "$pid" >/dev/null 2>&1 || true
  for ((i = 1; i <= 10; i++)); do
    pid_is_running "$pid" || return 0
    sleep 0.1
  done
  return 1
}

clear_pid_file_if_owned() {
  local pid_file="$1"
  local expected_pid="$2"
  local recorded_pid
  [[ -f "$pid_file" ]] || return 0
  recorded_pid="$(cat "$pid_file" 2>/dev/null || true)"
  if [[ "$recorded_pid" == "$expected_pid" ]]; then
    rm -f "$pid_file"
  fi
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

wait_for_port() {
  local host="$1"
  local port="$2"
  local name="$3"
  local attempts="${4:-60}"
  local i
  for ((i = 1; i <= attempts; i++)); do
    if port_is_listening "$host" "$port"; then
      log "$name is ready on $host:$port"
      return 0
    fi
    sleep 1
  done
  fail "$name did not become ready on $host:$port"
}

wait_for_port_closed() {
  local host="$1"
  local port="$2"
  local name="$3"
  local attempts="${4:-50}"
  local i
  for ((i = 1; i <= attempts; i++)); do
    if ! port_is_listening "$host" "$port"; then
      log "$name has stopped"
      return 0
    fi
    sleep 0.2
  done
  log "$name is still listening on $host:$port"
  return 1
}

write_runtime_env() {
  local env_file="$ZEALRAG_RUNTIME_DIR/zealrag.env"
  local system_key
  require_command openssl
  if [[ -f "$env_file" ]]; then
    chmod 600 "$env_file"
    return
  fi
  system_key="$(openssl rand -hex 16)"
  cat >"$env_file" <<EOF
SYSTEM_AES_KEY=$system_key
EOF
  chmod 600 "$env_file"
}
