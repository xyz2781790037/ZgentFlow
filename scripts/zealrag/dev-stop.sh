#!/usr/bin/env bash
set -Eeuo pipefail
source "$(dirname "$0")/common.sh"

stop_pid_file() {
  local name="$1"
  local pid_file="$2"
  local validator="$3"
  local attempts="${4:-50}"
  [[ -f "$pid_file" ]] || return 0

  local pid
  pid="$(cat "$pid_file")"
  [[ -n "$pid" ]] || return 0
  if pid_is_running "$pid"; then
    if "$validator" "$pid"; then
      stop_pid_gracefully "$name" "$pid" "$attempts" || true
    else
      log "ignoring stale $name PID file; pid $pid belongs to another process"
    fi
  fi
  clear_pid_file_if_owned "$pid_file" "$pid"
}

is_zealrag_supervisor() {
  pid_workdir_matches "$1" "$ZEALRAG_ROOT" && pid_command_contains "$1" "scripts/zealrag/dev-app.sh"
}

is_zealrag_backend() {
  pid_executable_matches "$1" "$ZEALRAG_RUNTIME_DIR/zealrag-server"
}

is_docreader() {
  pid_workdir_matches "$1" "$ZEALRAG_ROOT" && pid_command_contains "$1" "docreader.main"
}

stop_pid_file "ZgentFlow supervisor" "$ZEALRAG_RUNTIME_DIR/dev-app.pid" is_zealrag_supervisor 200
stop_pid_file "ZgentFlow backend" "$ZEALRAG_RUNTIME_DIR/backend.pid" is_zealrag_backend 75
stop_pid_file "docreader" "$ZEALRAG_RUNTIME_DIR/docreader.pid" is_docreader 50

wait_for_port_closed 127.0.0.1 "$ZEALRAG_APP_PORT" "ZgentFlow backend" 25 || true
wait_for_port_closed 127.0.0.1 "$ZEALRAG_DOCREADER_PORT" "docreader" 25 || true

if [[ -f "$ZEALRAG_RUNTIME_DIR/redis.pid" ]]; then
  redis_pid="$(cat "$ZEALRAG_RUNTIME_DIR/redis.pid")"
  if pid_is_running "$redis_pid" && pid_command_contains "$redis_pid" "redis-server"; then
    log "stopping Redis"
    redis-cli -h 127.0.0.1 -p "$ZEALRAG_REDIS_PORT" shutdown >/dev/null 2>&1 || true
  fi
  clear_pid_file_if_owned "$ZEALRAG_RUNTIME_DIR/redis.pid" "$redis_pid"
fi
wait_for_port_closed 127.0.0.1 "$ZEALRAG_REDIS_PORT" "Redis" 25 || true

if [[ -x "$ZEALRAG_RUNTIME_DIR/postgres/bin/pg_ctl" ]] &&    "$ZEALRAG_RUNTIME_DIR/postgres/bin/pg_ctl" --pgdata="$ZEALRAG_DATA_DIR/postgres" status >/dev/null 2>&1; then
  log "stopping PostgreSQL"
  "$ZEALRAG_RUNTIME_DIR/postgres/bin/pg_ctl" --pgdata="$ZEALRAG_DATA_DIR/postgres" stop -m fast >/dev/null
fi

log "local services stopped"
