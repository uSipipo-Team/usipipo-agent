#!/bin/bash
#
# WireGuard IP Allocation Load Test Runner
#
# This script runs all load test scenarios for the WireGuard IP allocation system.
# It can run individual scenarios or all scenarios in sequence.
#
# =============================================================================
# USAGE
# =============================================================================
#
# Run all tests:
#   ./run-load-tests.sh
#
# Run specific test:
#   ./run-load-tests.sh concurrent
#   ./run-load-tests.sh steady-state
#   ./run-load-tests.sh spike
#   ./run-load-tests.sh stress
#
# Run with InfluxDB output (for Grafana):
#   ./run-load-tests.sh --with-influxdb
#   ./run-load-tests.sh spike --with-influxdb
#
# Run with custom API URL:
#   API_URL=http://localhost:8080 ./run-load-tests.sh
#
# Run with custom timeout:
#   TIMEOUT_MS=60000 ./run-load-tests.sh
#
# =============================================================================
# PREREQUISITES
# =============================================================================
#
# 1. Install k6:
#    - macOS:    brew install k6
#    - Linux:    sudo apt-get install k6
#    - Windows:  choco install k6
#
# 2. Verify k6 installation:
#    k6 version
#
# =============================================================================
# SCENARIOS OVERVIEW
# =============================================================================
#
# 1. Concurrent Allocation Test (concurrent)
#    - 1000 virtual users creating VPN keys simultaneously
#    - Duration: ~30 seconds
#    - Target: p99 < 500ms, 0% duplicates, >99% success
#
# 2. Steady State Test (steady-state)
#    - 100 VUs sustained for 5 minutes
#    - Duration: ~6 minutes
#    - Target: p95 < 200ms, <1% errors
#
# 3. Spike Test (spike)
#    - 10 VUs -> 500 VUs spike -> 10 VUs
#    - Duration: ~2 minutes
#    - Target: Graceful handling of traffic spikes
#
# 4. Stress Test (stress)
#    - Gradually increases from 100 to 1000 VUs
#    - Duration: ~9 minutes
#    - Target: Identify breaking point
#
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Default configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_FILE="${SCRIPT_DIR}/wireguard-ip-allocation-test.js"
API_URL="${API_URL:-http://localhost:8080/api/v1}"
VPN_API_URL="${VPN_API_URL:-http://localhost:8080/api/v1/vpn}"
TIMEOUT_MS="${TIMEOUT_MS:-30000}"
WITH_INFLUXDB="${WITH_INFLUXDB:-false}"
INFLUXDB_URL="${INFLUXDB_URL:-http://localhost:8086}"
INFLUXDB_DB="${INFLUXDB_DB:-k6}"

# Usage information
usage() {
    echo "WireGuard IP Allocation Load Test Runner"
    echo ""
    echo "Usage: $0 [scenario] [options]"
    echo ""
    echo "Scenarios:"
    echo "  concurrent    - 1000 VUs concurrent allocation test"
    echo "  steady-state  - 100 VUs steady state for 5 minutes"
    echo "  spike         - Traffic spike test (10 -> 500 -> 10 VUs)"
    echo "  stress        - Stress test (gradual increase to 1000 VUs)"
    echo "  all          - Run all scenarios in sequence (default)"
    echo ""
    echo "Options:"
    echo "  --with-influxdb    Output metrics to InfluxDB"
    echo "  --api-url URL      Set the API URL"
    echo "  --timeout MS       Set request timeout in milliseconds"
    echo "  -h, --help         Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  API_URL           API base URL (default: ${API_URL})"
    echo "  VPN_API_URL       VPN API URL (default: ${VPN_API_URL})"
    echo "  TIMEOUT_MS        Request timeout (default: ${TIMEOUT_MS})"
    echo "  WITH_INFLUXDB     Enable InfluxDB output (default: false)"
    echo ""
    exit 0
}

# Parse arguments
SCENARIO="all"
while [[ $# -gt 0 ]]; do
    case $1 in
        concurrent|steady-state|spike|stress|all)
            SCENARIO="$1"
            shift
            ;;
        --with-influxdb)
            WITH_INFLUXDB="true"
            shift
            ;;
        --api-url)
            API_URL="$2"
            shift 2
            ;;
        --timeout)
            TIMEOUT_MS="$2"
            shift 2
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

# Print header
print_header() {
    local title="$1"
    local color="$2"
    echo ""
    echo -e "${color}========================================${NC}"
    echo -e "${color}${title}${NC}"
    echo -e "${color}========================================${NC}"
}

# Print test configuration
print_config() {
    echo ""
    echo "Configuration:"
    echo "  API URL:           ${API_URL}"
    echo "  VPN API URL:       ${VPN_API_URL}"
    echo "  Timeout:          ${TIMEOUT_MS}ms"
    echo "  InfluxDB Output:   ${WITH_INFLUXDB}"
    echo ""
}

# Build k6 command
build_k6_command() {
    local scenario="$1"
    local cmd="k6 run"
    
    # Add environment variables
    cmd+=" --env SCENARIO=${scenario}"
    cmd+=" --env ALLOCATION_API_URL=${API_URL}"
    cmd+=" --env VPN_API_URL=${VPN_API_URL}"
    cmd+=" --env TIMEOUT_MS=${TIMEOUT_MS}"
    
    # Add InfluxDB output if enabled
    if [ "${WITH_INFLUXDB}" = "true" ]; then
        cmd+=" --out influxdb=${INFLUXDB_URL}/write?db=${INFLUXDB_DB}"
    fi
    
    # Add test file
    cmd+=" ${TEST_FILE}"
    
    echo "$cmd"
}

# Run a single scenario
run_scenario() {
    local scenario="$1"
    local description="$2"
    
    print_header "${description}" "${CYAN}"
    print_config
    
    local cmd=$(build_k6_command "$scenario")
    echo "Running: ${cmd}"
    echo ""
    
    # Execute the test
    eval "$cmd"
    
    local exit_code=$?
    
    if [ $exit_code -eq 0 ]; then
        echo ""
        echo -e "${GREEN}[✓] ${scenario} test completed successfully${NC}"
    else
        echo ""
        echo -e "${RED}[✗] ${scenario} test failed with exit code ${exit_code}${NC}"
    fi
    
    return $exit_code
}

# Run all scenarios
run_all_scenarios() {
    local failed=0
    
    print_header "RUNNING ALL LOAD TEST SCENARIOS" "${BLUE}"
    print_config
    
    # Countdown
    echo "Starting in 3 seconds..."
    sleep 1
    echo "Starting in 2 seconds..."
    sleep 1
    echo "Starting in 1 second..."
    sleep 1
    echo ""
    
    # Run concurrent test
    if ! run_scenario "concurrent" "SCENARIO 1: CONCURRENT ALLOCATION TEST (1000 VUs)"; then
        failed=1
    fi
    echo ""
    echo "Press Enter to continue to next test..."
    read -r
    
    # Run steady state test
    if ! run_scenario "steady-state" "SCENARIO 2: STEADY STATE TEST (100 VUs for 5 minutes)"; then
        failed=1
    fi
    echo ""
    echo "Press Enter to continue to next test..."
    read -r
    
    # Run spike test
    if ! run_scenario "spike" "SCENARIO 3: SPIKE TEST (10 -> 500 -> 10 VUs)"; then
        failed=1
    fi
    echo ""
    echo "Press Enter to continue to next test..."
    read -r
    
    # Run stress test
    if ! run_scenario "stress" "SCENARIO 4: STRESS TEST (100 -> 1000 VUs)"; then
        failed=1
    fi
    
    return $failed
}

# Verify prerequisites
check_prerequisites() {
    echo -e "${CYAN}Checking prerequisites...${NC}"
    
    # Check if k6 is installed
    if ! command -v k6 &> /dev/null; then
        echo -e "${RED}Error: k6 is not installed${NC}"
        echo "Please install k6:"
        echo "  macOS:  brew install k6"
        echo "  Linux:  sudo apt-get install k6"
        echo "  Docker: docker run -it loadimpact/k6 run -"
        exit 1
    fi
    
    # Check if test file exists
    if [ ! -f "${TEST_FILE}" ]; then
        echo -e "${RED}Error: Test file not found at ${TEST_FILE}${NC}"
        exit 1
    fi
    
    # Check InfluxDB connectivity if enabled
    if [ "${WITH_INFLUXDB}" = "true" ]; then
        echo -e "${CYAN}Checking InfluxDB connectivity...${NC}"
        if ! curl -s "${INFLUXDB_URL}/ping" > /dev/null 2>&1; then
            echo -e "${YELLOW}Warning: InfluxDB not reachable at ${INFLUXDB_URL}${NC}"
            echo "Metrics will not be saved to InfluxDB"
            WITH_INFLUXDB="false"
        else
            echo -e "${GREEN}InfluxDB is reachable${NC}"
        fi
    fi
    
    echo -e "${GREEN}All prerequisites met${NC}"
    echo ""
}

# Main execution
main() {
    check_prerequisites
    
    case "${SCENARIO}" in
        all)
            run_all_scenarios
            ;;
        concurrent)
            run_scenario "concurrent" "CONCURRENT ALLOCATION TEST (1000 VUs)"
            ;;
        steady-state)
            run_scenario "steady-state" "STEADY STATE TEST (100 VUs for 5 minutes)"
            ;;
        spike)
            run_scenario "spike" "SPIKE TEST (10 -> 500 -> 10 VUs)"
            ;;
        stress)
            run_scenario "stress" "STRESS TEST (100 -> 1000 VUs)"
            ;;
    esac
    
    local exit_code=$?
    
    echo ""
    print_header "TEST RUN COMPLETE" "${BLUE}"
    
    if [ $exit_code -eq 0 ]; then
        echo -e "${GREEN}All tests completed successfully!${NC}"
        echo ""
        echo "To view detailed metrics in Grafana:"
        echo "  1. Open ${INFLUXDB_URL} in your browser"
        echo "  2. Create dashboards using the k6 metrics"
        echo ""
        echo "Key metrics to track:"
        echo "  - http_req_duration (p95, p99)"
        echo "  - ip_allocation_latency"
        echo "  - duplicate_ip_rate"
        echo "  - allocation_success_rate"
    else
        echo -e "${RED}Some tests failed. Please review the output above.${NC}"
    fi
    
    exit $exit_code
}

# Run main function
main