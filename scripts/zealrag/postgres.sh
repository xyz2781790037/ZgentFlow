#!/usr/bin/env bash
set -Eeuo pipefail
source "$(dirname "$0")/common.sh"

POSTGRES_HOME="$ZEALRAG_RUNTIME_DIR/postgres"
PGDATA="$ZEALRAG_DATA_DIR/postgres"
PGLOG="$ZEALRAG_LOG_DIR/postgres.log"
DB_USER="${DB_USER:-zealrag}"
DB_NAME="${DB_NAME:-zealrag_main}"

# The project-local PostgreSQL runtime is moved together with the repository.
# Resolve client shared libraries from its current location instead of the
# configure-time installation prefix.
export LD_LIBRARY_PATH="$POSTGRES_HOME/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"

[[ -x "$POSTGRES_HOME/bin/postgres" ]] || fail "PostgreSQL runtime is missing; run bootstrap-runtime.sh"
mkdir -p "$PGDATA"

if [[ ! -f "$PGDATA/PG_VERSION" ]]; then
  log "initializing PostgreSQL data directory"
  "$POSTGRES_HOME/bin/initdb" \
    --pgdata="$PGDATA" \
    --username="$DB_USER" \
    --encoding=UTF8 \
    --auth-local=trust \
    --auth-host=trust
fi

if "$POSTGRES_HOME/bin/pg_ctl" --pgdata="$PGDATA" status >/dev/null 2>&1; then
  log "PostgreSQL is already running"
else
  log "starting PostgreSQL on port $ZEALRAG_DB_PORT"
  "$POSTGRES_HOME/bin/pg_ctl" \
    --pgdata="$PGDATA" \
    --log="$PGLOG" \
    --options="-h 127.0.0.1 -p $ZEALRAG_DB_PORT -k $ZEALRAG_RUNTIME_DIR" \
    start
fi

wait_for_port 127.0.0.1 "$ZEALRAG_DB_PORT" PostgreSQL 60

if ! "$POSTGRES_HOME/bin/psql" -h 127.0.0.1 -p "$ZEALRAG_DB_PORT" -U "$DB_USER" -d postgres -Atqc \
  "SELECT 1 FROM pg_database WHERE datname = '$DB_NAME'" | grep -qx 1; then
  log "creating database $DB_NAME"
  "$POSTGRES_HOME/bin/createdb" -h 127.0.0.1 -p "$ZEALRAG_DB_PORT" -U "$DB_USER" "$DB_NAME"
fi

"$POSTGRES_HOME/bin/psql" -h 127.0.0.1 -p "$ZEALRAG_DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
  -v ON_ERROR_STOP=1 -qc 'CREATE EXTENSION IF NOT EXISTS vector'
