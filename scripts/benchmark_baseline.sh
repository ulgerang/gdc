#!/bin/bash
# P1 Performance Baseline Measurement Script
# Purpose: Establish performance baseline for gdc extract command
# Requirements: R9 (AC-R9-1) - Performance regression detection

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
GDC_BIN="${PROJECT_DIR}/gdc"
BASELINE_DIR="${PROJECT_DIR}/.opencode/weave/baselines"
BASELINE_FILE="${BASELINE_DIR}/p1_baseline.json"

# Number of runs
RUNS=10

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_header() {
    echo -e "${YELLOW}========================================"
    echo "  P1 Performance Baseline Measurement"
    echo -e "========================================${NC}"
    echo ""
}

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if gdc binary exists
check_prerequisites() {
    print_info "Checking prerequisites..."
    
    if [ ! -f "$GDC_BIN" ] && [ ! -f "${GDC_BIN}.exe" ]; then
        print_error "gdc binary not found at ${GDC_BIN}"
        echo "Please build gdc first: go build -o gdc ./cmd/gdc"
        exit 1
    fi
    
    if [ ! -d "${PROJECT_DIR}/.gdc" ]; then
        print_error ".gdc directory not found. Please run 'gdc init' first."
        exit 1
    fi
    
    # Ensure baseline directory exists
    mkdir -p "$BASELINE_DIR"
    
    print_info "Prerequisites check passed ✓"
    echo ""
}

# Get a sample node to test
get_sample_node() {
    # Find first available node
    NODE=$("$GDC_BIN" list --format minimal | head -1)
    if [ -z "$NODE" ]; then
        print_error "No nodes found in the project"
        exit 1
    fi
    echo "$NODE"
}

# Run performance measurement
measure_performance() {
    local node=$1
    local times=()
    
    print_info "Running ${RUNS} iterations for node: ${node}"
    echo ""
    
    echo "  Iteration | Time (ms)"
    echo "  --------------------"
    
    for i in $(seq 1 $RUNS); do
        # Measure time in milliseconds
        start_time=$(date +%s%N)
        
        # Run gdc extract (suppress output)
        if ! "$GDC_BIN" extract "$node" > /dev/null 2>&1; then
            print_error "gdc extract failed on iteration $i"
            exit 1
        fi
        
        end_time=$(date +%s%N)
        
        # Calculate duration in milliseconds
        duration=$(( (end_time - start_time) / 1000000 ))
        times+=($duration)
        
        printf "  %9d | %7d\n" "$i" "$duration"
    done
    
    echo ""
    
    # Calculate statistics
    calculate_stats "${times[@]}"
}

# Calculate statistics
calculate_stats() {
    local times=("$@")
    local sum=0
    local min=${times[0]}
    local max=${times[0]}
    
    for t in "${times[@]}"; do
        sum=$((sum + t))
        [ "$t" -lt "$min" ] && min=$t
        [ "$t" -gt "$max" ] && max=$t
    done
    
    local avg=$((sum / ${#times[@]}))
    
    # Sort times for percentile calculation
    IFS=$'\n' sorted_times=($(sort -n <<<"${times[*]}")); unset IFS
    
    # Calculate p95 (90th percentile for 10 runs, index 9)
    local p95_idx=$(( ${#times[@]} * 95 / 100 ))
    [ $p95_idx -ge ${#times[@]} ] && p95_idx=$(( ${#times[@]} - 1 ))
    local p95=${sorted_times[$p95_idx]}
    
    # Calculate p50 (median)
    local median_idx=$(( ${#times[@]} / 2 ))
    local p50=${sorted_times[$median_idx]}
    
    echo "  Statistics:"
    echo "  -----------"
    printf "  Min:    %6d ms\n" "$min"
    printf "  Max:    %6d ms\n" "$max"
    printf "  Avg:    %6d ms\n" "$avg"
    printf "  Median: %6d ms\n" "$p50"
    printf "  P95:    %6d ms\n" "$p95"
    echo ""
    
    # Save baseline
    save_baseline "$min" "$max" "$avg" "$p50" "$p95" "${times[@]}"
}

# Save baseline to JSON file
save_baseline() {
    local min=$1
    local max=$2
    local avg=$3
    local p50=$4
    local p95=$5
    shift 5
    local times=("$@")
    
    local timestamp
    timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    
    # Build times array JSON
    local times_json="["
    local first=true
    for t in "${times[@]}"; do
        if [ "$first" = true ]; then
            first=false
        else
            times_json+=","
        fi
        times_json+="$t"
    done
    times_json+="]"
    
    cat > "$BASELINE_FILE" << EOF
{
  "phase": "P1",
  "description": "gdc extract command performance baseline",
  "timestamp": "$timestamp",
  "runs": $RUNS,
  "statistics": {
    "min_ms": $min,
    "max_ms": $max,
    "avg_ms": $avg,
    "median_ms": $p50,
    "p95_ms": $p95
  },
  "times_ms": $times_json,
  "threshold": {
    "max_regression_percent": 10,
    "max_acceptable_p95_ms": $(( p95 * 110 / 100 ))
  }
}
EOF
    
    print_info "Baseline saved to: ${BASELINE_FILE}"
    echo ""
    print_info "Current P95: ${p95}ms"
    print_info "Max acceptable (10% regression): $(( p95 * 110 / 100 ))ms"
}

# Main execution
main() {
    print_header
    check_prerequisites
    
    print_info "Selecting sample node..."
    SAMPLE_NODE=$(get_sample_node)
    print_info "Using node: ${SAMPLE_NODE}"
    echo ""
    
    measure_performance "$SAMPLE_NODE"
    
    echo -e "${GREEN}========================================"
    echo "  Baseline measurement complete!"
    echo -e "========================================${NC}"
    echo ""
    echo "Baseline file: ${BASELINE_FILE}"
    echo ""
    echo "To verify this baseline, run:"
    echo "  ./scripts/benchmark_baseline.sh"
}

main "$@"
