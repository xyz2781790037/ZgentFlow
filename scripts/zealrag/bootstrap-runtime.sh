#!/usr/bin/env bash
set -Eeuo pipefail
source "$(dirname "$0")/common.sh"

POSTGRES_VERSION="${POSTGRES_VERSION:-17.5}"
PGVECTOR_VERSION="${PGVECTOR_VERSION:-0.8.0}"
UV_VERSION="${UV_VERSION:-0.8.17}"
POSTGRES_HOME="$ZEALRAG_RUNTIME_DIR/postgres"
DOWNLOAD_DIR="$ZEALRAG_RUNTIME_DIR/downloads"
SOURCE_DIR="$ZEALRAG_RUNTIME_DIR/src"
UV_HOME="$ZEALRAG_RUNTIME_DIR/uv"
TOOLCHAIN_HOME="$ZEALRAG_RUNTIME_DIR/toolchain"

require_command curl
require_command tar
require_command make
require_command gcc
require_command perl

mkdir -p "$DOWNLOAD_DIR" "$SOURCE_DIR" "$UV_HOME" "$TOOLCHAIN_HOME"
write_runtime_env

if ! command -v bison >/dev/null 2>&1 || ! command -v flex >/dev/null 2>&1 || ! command -v m4 >/dev/null 2>&1; then
  require_command apt-get
  require_command dpkg-deb
  if [[ ! -x "$TOOLCHAIN_HOME/usr/bin/bison" ]]; then
    log "downloading local bison/flex toolchain"
    (
      cd "$DOWNLOAD_DIR"
      apt-get download bison flex m4
      for package in bison_*.deb flex_*.deb m4_*.deb; do
        dpkg-deb -x "$package" "$TOOLCHAIN_HOME"
      done
    )
  fi
  export PATH="$TOOLCHAIN_HOME/usr/bin:$PATH"
  export M4="$TOOLCHAIN_HOME/usr/bin/m4"
  export BISON_PKGDATADIR="$TOOLCHAIN_HOME/usr/share/bison"
fi

if [[ ! -x "$POSTGRES_HOME/bin/postgres" ]]; then
  archive="$DOWNLOAD_DIR/postgresql-$POSTGRES_VERSION.tar.bz2"
  source="$SOURCE_DIR/postgresql-$POSTGRES_VERSION"
  if [[ ! -f "$archive" ]]; then
    log "downloading PostgreSQL $POSTGRES_VERSION"
    curl --fail --location --retry 3 \
      "https://ftp.postgresql.org/pub/source/v$POSTGRES_VERSION/postgresql-$POSTGRES_VERSION.tar.bz2" \
      --output "$archive.part"
    mv "$archive.part" "$archive"
  fi
  if [[ ! -x "$source/configure" ]]; then
    log "extracting PostgreSQL"
    tar -xjf "$archive" -C "$SOURCE_DIR"
  fi
  log "building PostgreSQL into $POSTGRES_HOME"
  (
    cd "$source"
    ./configure \
      --prefix="$POSTGRES_HOME" \
      --without-readline \
      --without-zlib \
      --without-icu \
      --with-uuid=e2fs
    make -C src/backend generated-headers
    make -j"${ZEALRAG_BUILD_JOBS:-4}"
    make install
  )
fi

if [[ ! -f "$POSTGRES_HOME/share/extension/uuid-ossp.control" ]]; then
  log "enabling PostgreSQL uuid support"
  (
    cd "$SOURCE_DIR/postgresql-$POSTGRES_VERSION"
    ./configure \
      --prefix="$POSTGRES_HOME" \
      --without-readline \
      --without-zlib \
      --without-icu \
      --with-uuid=e2fs
    make -C src/backend generated-headers
    make -j"${ZEALRAG_BUILD_JOBS:-4}"
    make install
    cd contrib/uuid-ossp
    make PG_CONFIG="$POSTGRES_HOME/bin/pg_config" -j"${ZEALRAG_BUILD_JOBS:-4}"
    make PG_CONFIG="$POSTGRES_HOME/bin/pg_config" install
  )
fi

if [[ ! -f "$POSTGRES_HOME/share/extension/pg_trgm.control" ]]; then
  log "building PostgreSQL pg_trgm extension"
  (
    cd "$SOURCE_DIR/postgresql-$POSTGRES_VERSION/contrib/pg_trgm"
    make PG_CONFIG="$POSTGRES_HOME/bin/pg_config" -j"${ZEALRAG_BUILD_JOBS:-4}"
    make PG_CONFIG="$POSTGRES_HOME/bin/pg_config" install
  )
fi

if [[ ! -f "$POSTGRES_HOME/lib/vector.so" ]]; then
  archive="$DOWNLOAD_DIR/pgvector-$PGVECTOR_VERSION.tar.gz"
  source="$SOURCE_DIR/pgvector-$PGVECTOR_VERSION"
  if [[ ! -f "$archive" ]]; then
    log "downloading pgvector $PGVECTOR_VERSION"
    curl --fail --location --retry 3 \
      "https://github.com/pgvector/pgvector/archive/refs/tags/v$PGVECTOR_VERSION.tar.gz" \
      --output "$archive.part"
    mv "$archive.part" "$archive"
  fi
  if [[ ! -f "$source/Makefile" ]]; then
    log "extracting pgvector"
    mkdir -p "$source"
    tar -xzf "$archive" -C "$source" --strip-components=1
  fi
  log "building pgvector"
  (
    cd "$source"
    make PG_CONFIG="$POSTGRES_HOME/bin/pg_config" -j"${ZEALRAG_BUILD_JOBS:-4}"
    make PG_CONFIG="$POSTGRES_HOME/bin/pg_config" install
  )
fi

if [[ ! -x "$UV_HOME/uv" ]]; then
  archive="$DOWNLOAD_DIR/uv-$UV_VERSION.tar.gz"
  if [[ ! -f "$archive" ]]; then
    log "downloading uv $UV_VERSION"
    curl --fail --location --retry 3 \
      "https://github.com/astral-sh/uv/releases/download/$UV_VERSION/uv-x86_64-unknown-linux-gnu.tar.gz" \
      --output "$archive.part"
    mv "$archive.part" "$archive"
  fi
  log "extracting uv"
  tar -xzf "$archive" -C "$UV_HOME" --strip-components=1
fi

log "runtime bootstrap complete"
