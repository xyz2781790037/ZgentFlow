#!/usr/bin/env bash
set -Eeuo pipefail
source "$(dirname "$0")/common.sh"

UV="$ZEALRAG_RUNTIME_DIR/uv/uv"
PID_FILE="$ZEALRAG_RUNTIME_DIR/docreader.pid"
LOG_FILE="$ZEALRAG_LOG_DIR/docreader.log"

[[ -x "$UV" ]] || fail "uv runtime is missing; run bootstrap-runtime.sh"

if [[ -f "$PID_FILE" ]] && kill -0 "$(cat "$PID_FILE")" >/dev/null 2>&1; then
  if (echo >"/dev/tcp/127.0.0.1/$ZEALRAG_DOCREADER_PORT") >/dev/null 2>&1; then
    log "docreader is already running"
    exit 0
  fi
  fail "docreader pid exists but port $ZEALRAG_DOCREADER_PORT is not ready"
fi

if (echo >"/dev/tcp/127.0.0.1/$ZEALRAG_DOCREADER_PORT") >/dev/null 2>&1; then
  fail "port $ZEALRAG_DOCREADER_PORT is already used by another service"
fi

log "installing docreader dependencies when needed"
UV_CACHE_DIR="$ZEALRAG_RUNTIME_DIR/uv-cache" "$UV" sync   --project "$ZEALRAG_ROOT/docreader"   --locked   --no-dev

PYTHON="$ZEALRAG_ROOT/docreader/.venv/bin/python"
[[ -x "$PYTHON" ]] || fail "docreader virtualenv python is missing after uv sync"

log "starting docreader on port $ZEALRAG_DOCREADER_PORT"
(
  cd "$ZEALRAG_ROOT"
  export PYTHONPATH="$ZEALRAG_ROOT"
  export DOCREADER_GRPC_PORT="$ZEALRAG_DOCREADER_PORT"
  export DOCREADER_GRPC_MAX_FILE_SIZE_MB=100
  export MAX_FILE_SIZE_MB=100
  export LOCAL_STORAGE_BASE_DIR="$ZEALRAG_DATA_DIR/files"
  nohup "$PYTHON" -m docreader.main >>"$LOG_FILE" 2>&1 &
  echo $! >"$PID_FILE"
)

wait_for_port 127.0.0.1 "$ZEALRAG_DOCREADER_PORT" docreader 120
