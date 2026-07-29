#!/usr/bin/env bash
set -Eeuo pipefail
source "$(dirname "$0")/common.sh"

REDIS_DATA_DIR="$ZEALRAG_DATA_DIR/redis"
REDIS_LOG="$ZEALRAG_LOG_DIR/redis.log"
REDIS_PID_FILE="$ZEALRAG_RUNTIME_DIR/redis.pid"

require_command redis-server
require_command redis-cli
mkdir -p "$REDIS_DATA_DIR"

if [[ -f "$REDIS_PID_FILE" ]]; then
  redis_pid="$(cat "$REDIS_PID_FILE" 2>/dev/null || true)"
  if pid_is_running "$redis_pid" && pid_command_contains "$redis_pid" "redis-server"; then
    log "Redis is already running"
  else
    clear_pid_file_if_owned "$REDIS_PID_FILE" "$redis_pid"
  fi
fi

if [[ ! -f "$REDIS_PID_FILE" ]]; then
  if port_is_listening 127.0.0.1 "$ZEALRAG_REDIS_PORT"; then
    fail "Redis port $ZEALRAG_REDIS_PORT is already in use"
  fi
  log "starting Redis on port $ZEALRAG_REDIS_PORT"
  redis-server \
    --bind 127.0.0.1 \
    --protected-mode yes \
    --port "$ZEALRAG_REDIS_PORT" \
    --dir "$REDIS_DATA_DIR" \
    --appendonly yes \
    --daemonize yes \
    --pidfile "$REDIS_PID_FILE" \
    --logfile "$REDIS_LOG"
fi

wait_for_port 127.0.0.1 "$ZEALRAG_REDIS_PORT" Redis 30
for ((i = 1; i <= 30; i++)); do
  if redis-cli -h 127.0.0.1 -p "$ZEALRAG_REDIS_PORT" ping 2>/dev/null | grep -qx PONG; then
    exit 0
  fi
  sleep 0.2
done
fail "Redis did not respond to PING on port $ZEALRAG_REDIS_PORT"
