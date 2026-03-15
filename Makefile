.PHONY: all build test test-p1 test-p3 test-p4 clean help

# Default target
all: build

# Build the gdc binary
build:
	go build -o gdc ./cmd/gdc

# Run all tests
test:
	go test ./...

# Run P1 phase validation tests
test-p1: build
	@echo "========================================"
	@echo "  P1 Phase Validation Tests"
	@echo "========================================"
	@echo ""
	@echo "1. Checking test fixtures exist..."
	@test -f fixtures/p1/sample.cs || (echo "  ✗ C# fixture missing" && exit 1)
	@echo "  ✓ C# fixture exists (fixtures/p1/sample.cs)"
	@test -f fixtures/p1/sample.ts || (echo "  ✗ TypeScript fixture missing" && exit 1)
	@echo "  ✓ TypeScript fixture exists (fixtures/p1/sample.ts)"
	@test -f fixtures/p1/sample.go || (echo "  ✗ Go fixture missing" && exit 1)
	@echo "  ✓ Go fixture exists (fixtures/p1/sample.go)"
	@echo ""
	@echo "2. Checking baseline script..."
	@test -f scripts/benchmark_baseline.sh || (echo "  ✗ Baseline script missing" && exit 1)
	@echo "  ✓ Baseline script exists (scripts/benchmark_baseline.sh)"
	@echo ""
	@echo "3. Checking gdc binary..."
	@test -f gdc || (echo "  ✗ gdc binary missing, building..." && go build -o gdc ./cmd/gdc)
	@echo "  ✓ gdc binary exists"
	@echo ""
	@echo "========================================"
	@echo "  All P1 tests passed! ✓"
	@echo "========================================"
	@echo ""
	@echo "To establish performance baseline, run:"
	@echo "  bash scripts/benchmark_baseline.sh"

# Run P3 phase validation tests (Parser Enhancement)
test-p3: build
	@echo "========================================"
	@echo "  P3 Phase Validation Tests"
	@echo "  Parser Enhancement (C#/TypeScript)"
	@echo "========================================"
	@echo ""
	@echo "1. Running parser unit tests..."
	@go test ./internal/parser/... -v -count=1
	@echo ""
	@echo "2. Running P3 integration tests..."
	@go test ./tests/integration/... -v -count=1
	@echo ""
	@echo "========================================"
	@echo "  All P3 tests passed! ✓"
	@echo "========================================"

# Run P4 phase validation tests (Search and Query Commands)
test-p4: build
	@echo "========================================"
	@echo "  P4 Phase Validation Tests"
	@echo "  Search, Query, and Trace Commands"
	@echo "========================================"
	@echo ""
	@echo "1. Running search tests..."
	@go test ./tests/integration/... -v -count=1 -run TestSearch
	@echo ""
	@echo "2. Running query tests..."
	@go test ./tests/integration/... -v -count=1 -run TestQuery
	@echo ""
	@echo "3. Running trace tests..."
	@go test ./tests/integration/... -v -count=1 -run TestTrace
	@echo ""
	@echo "4. Running graceful degradation tests..."
	@go test ./tests/integration/... -v -count=1 -run "TestGraceful|TestVersion|TestList|TestHelp"
	@echo ""
	@echo "========================================"
	@echo "  All P4 tests passed! ✓"
	@echo "========================================"

# Clean build artifacts
clean:
	rm -f gdc
	rm -f gdc.exe
	go clean

# Show help
help:
	@echo "Available targets:"
	@echo "  build     - Build the gdc binary"
	@echo "  test      - Run all Go tests"
	@echo "  test-p1   - Run P1 phase validation tests"
	@echo "  test-p3   - Run P3 phase validation tests"
	@echo "  test-p4   - Run P4 phase validation tests (Search/Query/Trace)"
	@echo "  clean     - Clean build artifacts"
	@echo "  help      - Show this help message"
