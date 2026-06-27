ZEALRAG_APP_PORT ?= 8081

.PHONY: help build test dev-app dev-frontend dev-stop bootstrap-runtime

help:
	@echo "ZgentFlow development commands"
	@echo "  make dev-app          Bootstrap dependencies and start the backend"
	@echo "  make dev-frontend     Start the Vue development server"
	@echo "  make bootstrap-runtime Download/build local Linux amd64 runtimes"
	@echo "  make dev-stop         Stop project-local backend dependencies"
	@echo "  make build            Build the ZgentFlow backend"
	@echo "  make test             Run Go tests"

build:
	go build -o .runtime/zealrag-server ./cmd/server

test:
	go test ./...

bootstrap-runtime:
	./scripts/zealrag/bootstrap-runtime.sh

dev-app:
	ZEALRAG_APP_PORT=$(ZEALRAG_APP_PORT) ./scripts/zealrag/dev-app.sh

dev-frontend:
	cd frontend && FRONTEND_BACKEND_URL=$${FRONTEND_BACKEND_URL:-http://127.0.0.1:$(ZEALRAG_APP_PORT)} npm run dev -- --host 0.0.0.0

dev-stop:
	./scripts/zealrag/dev-stop.sh
