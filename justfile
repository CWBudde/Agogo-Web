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
#
# The golangci-lint invocation deliberately matches .github/workflows/test-go-lint.yml,
# which runs the linter with its defaults over the whole module. It used to pass
# `--tests=false ./internal/...` here, so `just lint` was green while CI was red on
# any finding in a _test.go file or outside internal/ — a local gate that cannot
# fail where CI does is worse than no local gate.
lint:
    cd packages/engine-wasm && go vet ./...
    cd packages/engine-wasm && GOCACHE=$(mktemp -d) GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run --timeout=5m ./...
    bun run --cwd apps/editor-web lint

# Auto-fix all lint issues
lint-fix:
    cd packages/engine-wasm && GOCACHE=$(mktemp -d) GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run --fix --timeout=5m ./...
    bun run --cwd apps/editor-web lint:fix

# ── Combined ──────────────────────────────────────────────────────────────────

# Run all tests
test: test-go fe-typecheck fe-test

# Full production build (wasm + frontend)
build: wasm-build fe-build

# Run all CI checks
ci: check-formatted test lint check-tidy build

# ── PSD fixtures ──────────────────────────────────────────────────────────────
#
# Tooling lives in tools/psdfixtures/ and needs Python + psd-tools + pytoshop
# (and, for the flat fixtures, ImageMagick). Run `just fixtures-setup` once to
# build the pinned venv from tools/psdfixtures/requirements.txt, then pass its
# interpreter to the recipes below, e.g.
#
#     just fixtures-generate .fixtures-out .venv-psdfixtures/bin/python
#
# These recipes are DELIBERATELY not part of `test` or `ci`: `just ci` and
# `go test` must stay Go-only and must never acquire a Python dependency. The
# fixture binaries and their sidecars are committed, so regenerating them is a
# maintenance task, not a build step — and fixtures-setup is therefore not a
# prerequisite of anything.

# Create the pinned Python venv the fixture scripts need (once, not per build)
fixtures-setup venv=".venv-psdfixtures":
    python3 -m venv {{ venv }}
    {{ venv }}/bin/python -m pip install --upgrade pip
    {{ venv }}/bin/python -m pip install -r tools/psdfixtures/requirements.txt
    @{{ venv }}/bin/python -c "import pytoshop, psd_tools; print('pytoshop', pytoshop.__version__, '/ psd-tools', psd_tools.__version__)"
    @echo "Run the fixture scripts with {{ venv }}/bin/python, e.g."
    @echo "  {{ venv }}/bin/python tools/psdfixtures/generate_pytoshop.py --out .fixtures-out"

# Generate PSD fixture binaries into a scratch directory (not the committed corpus)
fixtures-generate outdir=".fixtures-out" python="python3":
    tools/psdfixtures/generate_imagemagick.sh {{ outdir }}
    {{ python }} tools/psdfixtures/generate_pytoshop.py --out {{ outdir }}
    @echo "Fixtures written to {{ outdir }} (ImageMagick: flat; pytoshop: layered)."
    @echo "Then derive sidecars with tools/psdfixtures/derive_expectations.py."

# Re-read Agogo-written PSDs with psd-tools, an independent reader
fixtures-verify dir="packages/engine-wasm/internal/io/psdfixture/_dump" python="python3":
    {{ python }} tools/psdfixtures/verify_dump.py {{ dir }} --identify

# ── Cleanup ───────────────────────────────────────────────────────────────────

# Remove all build artifacts
clean:
    rm -f apps/editor-web/public/engine.wasm
    rm -f apps/editor-web/public/wasm_exec.js
    rm -rf apps/editor-web/dist
    rm -f packages/engine-wasm/coverage.out packages/engine-wasm/coverage.html

fix:
    just lint-fix
    just fmt
