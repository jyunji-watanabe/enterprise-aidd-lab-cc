# Single entry point for building, running and checking the project.
SHELL := /bin/bash
export GOTOOLCHAIN ?= local

BACKEND  := backend
FRONTEND := frontend
DB       ?= $(BACKEND)/data/expense.db
ADDR     ?= :8080

.PHONY: help setup build run dev-backend dev-frontend seed remind reset-db \
        lint lint-backend lint-frontend format typecheck \
        test test-backend test-frontend coverage e2e check clean docker-up

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

setup: ## Install dependencies (Go modules, npm packages)
	cd $(BACKEND) && go mod download
	cd $(FRONTEND) && npm ci

build: ## Build the frontend bundle and the backend binary (bin/expense)
	cd $(FRONTEND) && npm run build
	cd $(BACKEND) && go build -o ../bin/expense ./cmd/expense

run: build ## Build and start the app on http://localhost:8080 (seeds an empty DB)
	./bin/expense serve -addr $(ADDR) -db $(DB) -static $(FRONTEND)/dist

dev-backend: ## Run the API only (use with dev-frontend for hot reload)
	cd $(BACKEND) && go run ./cmd/expense serve -addr $(ADDR) -db data/expense.db

dev-frontend: ## Run the Vite dev server on :5173 (proxies /api to :8080)
	cd $(FRONTEND) && npm run dev

seed: ## Seed the database if it is empty
	cd $(BACKEND) && go run ./cmd/expense seed -db data/expense.db

remind: ## Run the unapproved-claims reminder batch once (CLI)
	cd $(BACKEND) && go run ./cmd/expense remind -db data/expense.db

reset-db: ## Delete the local database (it is re-seeded on next start)
	rm -f $(BACKEND)/data/expense.db*

lint: lint-backend lint-frontend ## Run all linters / static analysis

lint-backend: ## golangci-lint (govet, staticcheck, gosec, errcheck, revive, ...)
	cd $(BACKEND) && go vet ./... && golangci-lint run ./...

lint-frontend: ## ESLint (typescript-eslint strict, type-aware) + Prettier check
	cd $(FRONTEND) && npm run lint && npm run format:check

typecheck: ## TypeScript type check
	cd $(FRONTEND) && npm run typecheck

format: ## Auto-format Go and TS code
	cd $(BACKEND) && gofmt -w .
	cd $(FRONTEND) && npm run format

test: test-backend test-frontend ## Run all unit / integration tests

test-backend: ## Go unit + integration tests (race detector)
	cd $(BACKEND) && go test -race -count=1 ./...

test-frontend: ## Vitest unit/component tests
	cd $(FRONTEND) && npm test

coverage: ## Test coverage reports
	cd $(BACKEND) && go test -count=1 -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1
	cd $(FRONTEND) && npm run test:coverage

e2e: ## Playwright end-to-end tests against the real server (fresh seeded DB)
	cd $(FRONTEND) && npm run e2e

check: lint typecheck test e2e ## Everything CI runs

docker-up: ## Build and start with docker compose
	docker compose up --build

clean: ## Remove build outputs
	rm -rf bin $(FRONTEND)/dist $(FRONTEND)/coverage $(FRONTEND)/test-results $(FRONTEND)/playwright-report $(BACKEND)/coverage.out
