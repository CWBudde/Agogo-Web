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
    GOROOT=$(go env GOROOT) && \
        if [ -f "$GOROOT/lib/wasm/wasm_exec.js" ]; then \
            cp "$GOROOT/lib/wasm/wasm_exec.js" apps/editor-web/public/wasm_exec.js; \
        elif [ -f "$GOROOT/misc/wasm/wasm_exec.js" ]; then \
            cp "$GOROOT/misc/wasm/wasm_exec.js" apps/editor-web/public/wasm_exec.js; \
        else \
            echo "ERROR: wasm_exec.js not found in GOROOT" && exit 1; \
        fi

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

# Run frontend unit tests
fe-test:
    bun run --cwd apps/editor-web test

# ── Go / Engine ───────────────────────────────────────────────────────────────

# Run Go unit tests on host (no Wasm target needed)
# cmd/engine is excluded: its files are js+wasm only and can't be compiled on the host.
test-go:
    cd packages/engine-wasm && go test $(go list ./... | grep -v 'cmd/engine')

# Run Go tests with race detector
test-go-race:
    cd packages/engine-wasm && go test -race $(go list ./... | grep -v 'cmd/engine')

# Run Go tests with coverage report
test-go-coverage:
    cd packages/engine-wasm && go test -v -coverprofile=coverage.out $(go list ./... | grep -v 'cmd/engine')
    cd packages/engine-wasm && go tool cover -html=coverage.out -o coverage.html

# Update golden test snapshots
update-golden:
    cd packages/engine-wasm && UPDATE_GOLDEN=1 go test $(go list ./... | grep -v 'cmd/engine')

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
    cd packages/engine-wasm && GOCACHE=$(mktemp -d) GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run --tests=false --timeout=2m ./internal/...
    bun run --cwd apps/editor-web lint

# Auto-fix all lint issues
lint-fix:
    cd packages/engine-wasm && GOCACHE=$(mktemp -d) GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run --fix --tests=false --timeout=2m ./internal/...
    bun run --cwd apps/editor-web lint:fix

# ── Combined ──────────────────────────────────────────────────────────────────

# Run all tests
test: test-go fe-typecheck fe-test

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
