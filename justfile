set shell := ["bash", "-uc"]

# Default recipe - show available commands
default:
    @just --list

# ── Wasm ──────────────────────────────────────────────────────────────────────

# Build engine.wasm + copy wasm_exec.js into editor-web/public/
wasm-build:
    mkdir -p apps/editor-web/public
    GOOS=js GOARCH=wasm go build -C packages/engine-wasm \
        -ldflags="-s -w -X github.com/MeKo-Tech/agogo-web/packages/engine-wasm/internal/buildinfo.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        -o ../../apps/editor-web/public/engine.wasm \
        ./cmd/engine
    cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" apps/editor-web/public/wasm_exec.js

# ── Frontend ──────────────────────────────────────────────────────────────────

# Install all workspace dependencies and git hooks
install:
    bun install
    bun run lefthook install

# Start Vite dev server (builds wasm first)
dev: wasm-build
    bun run --cwd apps/editor-web dev

# Build frontend for production
fe-build:
    bun run --cwd apps/editor-web build

# Run TypeScript type-check
fe-typecheck:
    bun run --cwd apps/editor-web typecheck

# ── Go / Engine ───────────────────────────────────────────────────────────────

# Run Go unit tests on host (no Wasm target needed)
test-go:
    cd packages/engine-wasm && go test ./...

# Run Go tests with race detector
test-go-race:
    cd packages/engine-wasm && go test -race ./...

# Run Go tests with coverage report
test-go-coverage:
    cd packages/engine-wasm && go test -v -coverprofile=coverage.out ./...
    cd packages/engine-wasm && go tool cover -html=coverage.out -o coverage.html

# Update golden test snapshots
update-golden:
    cd packages/engine-wasm && UPDATE_GOLDEN=1 go test ./...

# Ensure go.mod is tidy
check-tidy:
    cd packages/engine-wasm && go mod tidy
    git diff --exit-code packages/engine-wasm/go.mod packages/engine-wasm/go.sum

# ── Formatting ────────────────────────────────────────────────────────────────

# Format all code using treefmt + Biome
fmt:
    treefmt --allow-missing-formatter
    bun run --cwd apps/editor-web lint:fix

# Check if code is formatted correctly (CI-safe, no writes)
check-formatted:
    treefmt --allow-missing-formatter --fail-on-change

# ── Linting ───────────────────────────────────────────────────────────────────

# Run all linters
lint:
    cd packages/engine-wasm && go vet ./...
    cd packages/engine-wasm && golangci-lint run --timeout=2m ./...
    bun run --cwd apps/editor-web lint

# Auto-fix all lint issues
lint-fix:
    cd packages/engine-wasm && golangci-lint run --fix --timeout=2m ./...
    bun run --cwd apps/editor-web lint:fix

# ── Combined ──────────────────────────────────────────────────────────────────

# Run all tests
test: test-go fe-typecheck

# Full production build (wasm + frontend)
build: wasm-build fe-build

# Run all CI checks
ci: check-formatted test lint check-tidy build

# ── Cleanup ───────────────────────────────────────────────────────────────────

# Remove all build artifacts
clean:
    rm -f apps/editor-web/public/engine.wasm
    rm -f apps/editor-web/public/wasm_exec.js
    rm -rf apps/editor-web/dist
    rm -f packages/engine-wasm/coverage.out packages/engine-wasm/coverage.html
