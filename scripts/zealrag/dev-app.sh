#!/usr/bin/env bash
set -Eeuo pipefail
source "$(dirname "$0")/common.sh"

# Load developer-only credentials and overrides. The file is gitignored; keep
# production secrets in the deployment platform's secret manager instead.
if [[ -f "$ZEALRAG_ROOT/.env" ]]; then
  set -a
  source "$ZEALRAG_ROOT/.env"
  set +a
fi

APP_PID=""
DEV_APP_PID_FILE="$ZEALRAG_RUNTIME_DIR/dev-app.pid"
BACKEND_PID_FILE="$ZEALRAG_RUNTIME_DIR/backend.pid"
DEV_APP_LOCK_FILE="$ZEALRAG_RUNTIME_DIR/dev-app.lock"
SERVER_BIN="$ZEALRAG_RUNTIME_DIR/zealrag-server"
SIGNAL_RECEIVED=0

supervisor_process_is_ours() {
  local pid="$1"
  pid_workdir_matches "$pid" "$ZEALRAG_ROOT" &&
    pid_command_contains "$pid" "scripts/zealrag/dev-app.sh"
}

backend_process_is_ours() {
  pid_executable_matches "$1" "$SERVER_BIN"
}

fail_port_in_use() {
  local listener_pid="${1:-}"
  local owner="an unknown process"
  if [[ -n "$listener_pid" ]]; then
    owner="$(process_summary "$listener_pid")"
  fi
  fail "backend port $ZEALRAG_APP_PORT is already in use by $owner. Stop that PID or choose another port: make dev-app ZEALRAG_APP_PORT=8082"
}

ensure_backend_port_available() {
  local listener_pid=""
  if ! port_is_listening 127.0.0.1 "$ZEALRAG_APP_PORT"; then
    return 0
  fi

  listener_pid="$(listener_pid_for_port "$ZEALRAG_APP_PORT" || true)"
  if [[ -n "$listener_pid" ]] && backend_process_is_ours "$listener_pid"; then
    log "recovering project backend left on port $ZEALRAG_APP_PORT (pid $listener_pid)"
    stop_pid_gracefully "stale ZgentFlow backend" "$listener_pid" 75 || true
    clear_pid_file_if_owned "$BACKEND_PID_FILE" "$listener_pid"
    wait_for_port_closed 127.0.0.1 "$ZEALRAG_APP_PORT" "stale ZgentFlow backend" 75 ||
      fail "project backend pid $listener_pid could not release port $ZEALRAG_APP_PORT"
    return 0
  fi

  fail_port_in_use "$listener_pid"
}

wait_for_backend_ready() {
  local i listener_pid exit_status
  for ((i = 1; i <= 100; i++)); do
    if ! pid_is_running "$APP_PID"; then
      set +e
      wait "$APP_PID"
      exit_status=$?
      set -e
      fail "ZgentFlow backend exited before becoming ready (exit $exit_status)"
    fi
    if port_is_listening 127.0.0.1 "$ZEALRAG_APP_PORT"; then
      listener_pid="$(listener_pid_for_port "$ZEALRAG_APP_PORT" || true)"
      if [[ -z "$listener_pid" || "$listener_pid" == "$APP_PID" ]]; then
        log "ZgentFlow backend is ready at http://127.0.0.1:$ZEALRAG_APP_PORT"
        return 0
      fi
      fail_port_in_use "$listener_pid"
    fi
    sleep 0.2
  done
  fail "ZgentFlow backend did not become ready on port $ZEALRAG_APP_PORT"
}

if command -v flock >/dev/null 2>&1; then
  exec 9>"$DEV_APP_LOCK_FILE"
  flock -n 9 || fail "make dev-app is already running; stop it with Ctrl+C or run make dev-stop"
else
  log "warning: flock is unavailable; falling back to PID-file startup protection"
fi

if [[ -f "$DEV_APP_PID_FILE" ]]; then
  existing_pid="$(cat "$DEV_APP_PID_FILE")"
  if pid_is_running "$existing_pid" && supervisor_process_is_ours "$existing_pid"; then
    fail "make dev-app is already running (pid $existing_pid); stop it with Ctrl+C before starting another instance"
  fi
fi

ensure_backend_port_available

printf '%s\n' "$$" >"$DEV_APP_PID_FILE"

cleanup() {
  local status=$?
  local listener_pid=""
  set +e
  trap - EXIT INT TERM
  if [[ "$SIGNAL_RECEIVED" == "1" ]]; then
    status=0
  fi

  if [[ -n "${APP_PID:-}" ]] && pid_is_running "$APP_PID"; then
    stop_pid_gracefully "ZgentFlow backend" "$APP_PID" 75 || true
    wait "$APP_PID" >/dev/null 2>&1 || true
  fi
  if ! wait_for_port_closed 127.0.0.1 "$ZEALRAG_APP_PORT" "ZgentFlow backend" 75; then
    listener_pid="$(listener_pid_for_port "$ZEALRAG_APP_PORT" || true)"
    if [[ -n "$listener_pid" ]] && backend_process_is_ours "$listener_pid"; then
      stop_pid_gracefully "remaining ZgentFlow backend" "$listener_pid" 25 || true
      wait_for_port_closed 127.0.0.1 "$ZEALRAG_APP_PORT" "ZgentFlow backend" 25 || true
    elif [[ -n "$listener_pid" ]]; then
      log "port $ZEALRAG_APP_PORT is now owned by $(process_summary "$listener_pid"); leaving it untouched"
    fi
  fi
  if [[ -n "${APP_PID:-}" ]]; then
    clear_pid_file_if_owned "$BACKEND_PID_FILE" "$APP_PID"
  fi

  if [[ -f "$ZEALRAG_RUNTIME_DIR/docreader.pid" ]]; then
    pid="$(cat "$ZEALRAG_RUNTIME_DIR/docreader.pid")"
    if pid_is_running "$pid" && pid_command_contains "$pid" "docreader.main"; then
      stop_pid_gracefully "docreader" "$pid" 50 || true
    fi
    clear_pid_file_if_owned "$ZEALRAG_RUNTIME_DIR/docreader.pid" "$pid"
  fi
  wait_for_port_closed 127.0.0.1 "$ZEALRAG_DOCREADER_PORT" "docreader" 50 || true

  if [[ -f "$ZEALRAG_RUNTIME_DIR/redis.pid" ]]; then
    pid="$(cat "$ZEALRAG_RUNTIME_DIR/redis.pid")"
    if pid_is_running "$pid" && pid_command_contains "$pid" "redis-server"; then
      log "stopping Redis"
      redis-cli -h 127.0.0.1 -p "$ZEALRAG_REDIS_PORT" shutdown >/dev/null 2>&1 || true
    fi
    clear_pid_file_if_owned "$ZEALRAG_RUNTIME_DIR/redis.pid" "$pid"
  fi
  wait_for_port_closed 127.0.0.1 "$ZEALRAG_REDIS_PORT" "Redis" 50 || true

  if [[ -x "$ZEALRAG_RUNTIME_DIR/postgres/bin/pg_ctl" ]] &&      "$ZEALRAG_RUNTIME_DIR/postgres/bin/pg_ctl" --pgdata="$ZEALRAG_DATA_DIR/postgres" status >/dev/null 2>&1; then
    log "stopping PostgreSQL"
    "$ZEALRAG_RUNTIME_DIR/postgres/bin/pg_ctl" --pgdata="$ZEALRAG_DATA_DIR/postgres" stop -m fast >/dev/null 2>&1 || true
  fi
  clear_pid_file_if_owned "$DEV_APP_PID_FILE" "$$"
  exit "$status"
}
trap cleanup EXIT
trap 'SIGNAL_RECEIVED=1; exit 0' INT TERM

"$(dirname "$0")/bootstrap-runtime.sh"
"$(dirname "$0")/postgres.sh"
"$(dirname "$0")/redis.sh"
"$(dirname "$0")/docreader.sh"

write_runtime_env
set -a
source "$ZEALRAG_RUNTIME_DIR/zealrag.env"
set +a

export SERVER_PORT="$ZEALRAG_APP_PORT"
export GIN_MODE="${GIN_MODE:-debug}"
export DB_DRIVER=postgres
export STORAGE_ALLOW_LIST=local
export STREAM_MANAGER_TYPE=memory
export DB_HOST=127.0.0.1
export DB_PORT="$ZEALRAG_DB_PORT"
export DB_USER="${DB_USER:-zealrag}"
export DB_PASSWORD="${DB_PASSWORD:-zealrag-local}"
export DB_NAME="${DB_NAME:-zealrag_main}"
export REDIS_ADDR="127.0.0.1:$ZEALRAG_REDIS_PORT"
export DOCREADER_ADDR="127.0.0.1:$ZEALRAG_DOCREADER_PORT"
export DOCREADER_TRANSPORT=grpc
export MAX_FILE_SIZE_MB=100
export LOCAL_STORAGE_BASE_DIR="$ZEALRAG_DATA_DIR/files"
# Proxy DNS implementations such as Clash may resolve public model APIs into
# 198.18.0.0/15. Trust ZgentFlow's built-in providers while preserving any
# deployment-specific hosts supplied by the user.
BUILTIN_MODEL_API_HOSTS="api.siliconflow.cn,api.deepseek.com,api.hunyuan.cloud.tencent.com"
if [[ -n "${SSRF_WHITELIST_EXTRA:-}" ]]; then
  export SSRF_WHITELIST_EXTRA="$BUILTIN_MODEL_API_HOSTS,$SSRF_WHITELIST_EXTRA"
else
  export SSRF_WHITELIST_EXTRA="$BUILTIN_MODEL_API_HOSTS"
fi
unset REDIS_PASSWORD REDIS_USERNAME

mkdir -p "$LOCAL_STORAGE_BASE_DIR"
cd "$ZEALRAG_ROOT"

log "building ZgentFlow backend"
LDFLAGS="-X 'google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=warn'"
go build -ldflags="$LDFLAGS" -o "$SERVER_BIN" ./cmd/server

ensure_backend_port_available
log "starting ZgentFlow backend at http://127.0.0.1:$ZEALRAG_APP_PORT"
"$SERVER_BIN" &
APP_PID=$!
printf '%s\n' "$APP_PID" >"$BACKEND_PID_FILE"
wait_for_backend_ready

set +e
wait "$APP_PID"
backend_status=$?
set -e
exit "$backend_status"
