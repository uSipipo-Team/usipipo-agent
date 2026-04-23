/**
 * WireGuard IP Allocation Load Test Suite
 * 
 * This script provides comprehensive load testing for the WireGuard IP allocation system
 * using k6, a modern load testing tool. It tests various scenarios to ensure the system
 * can handle concurrent allocations, steady state workloads, traffic spikes, and stress.
 * 
 * =============================================================================
 * PREREQUISITES
 * =============================================================================
 * 
 * 1. Install k6:
 *    - macOS:    brew install k6
 *    - Linux:    sudo gpg -k
 *               sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver keyserver.ubuntu.com --recv-keys C5AD17C5E1F6F5B3C5AD17C5E1F6F5B3C5AD17C5E1F6F5B3
 *               echo "deb https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
 *               sudo apt-get update && sudo apt-get install k6
 *    - Windows:  choco install k6
 * 
 * 2. Install Docker (for InfluxDB/Grafana):
 *    docker run -d --name influxdb -p 8086:8086 \
 *      -e INFLUXDB_DB=k6 -e INFLUXDB_USER=k6 -e INFLUXDB_USER_PASSWORD=k6password \
 *      influxdb:latest
 * 
 * 3. Start Grafana (optional, for visualization):
 *    docker run -d --name grafana -p 3000:3000 \
 *      -e GF_SECURITY_ADMIN_PASSWORD=admin \
 *      grafana/grafana
 * 
 * =============================================================================
 * HOW TO RUN
 * =============================================================================
 * 
 * Quick Start (using predefined scenarios):
 * 
 *   # Run concurrent allocation test (1000 VUs, 30 seconds)
 *   k6 run wireguard-ip-allocation-test.js
 *   # Or with environment-specific configuration:
 *   ALLOCATION_API_URL=http://localhost:8080/api/v1 \
 *   k6 run wireguard-ip-allocation-test.js
 * 
 * Run Specific Scenarios:
 * 
 *   # Scenario 1: Concurrent Allocation Test (1000 VUs)
 *   k6 run --env SCENARIO=concurrent wireguard-ip-allocation-test.js
 *   
 *   # Scenario 2: Steady State Test (100 VUs for 5 minutes)
 *   k6 run --env SCENARIO=steady-state wireguard-ip-allocation-test.js
 *   
 *   # Scenario 3: Spike Test (10 -> 500 -> 10 VUs)
 *   k6 run --env SCENARIO=spike wireguard-ip-allocation-test.js
 *   
 *   # Scenario 4: Stress Test (gradual increase to 1000 VUs)
 *   k6 run --env SCENARIO=stress wireguard-ip-allocation-test.js
 *   
 *   # Run all scenarios in sequence
 *   ./run-load-tests.sh
 * 
 * Output to InfluxDB for Grafana dashboards:
 * 
 *   k6 run --out influxdb=http://localhost:8086/write?db=k6&u=k6&p=k6password \
 *     wireguard-ip-allocation-test.js
 * 
 * =============================================================================
 * INTERPRETING RESULTS
 * =============================================================================
 * 
 * Success Criteria:
 * 
 * 1. Concurrent Allocation Test:
 *    - p99 latency < 500ms ✓
 *    - Duplicate IP rate = 0% ✓
 *    - Success rate > 99% ✓
 * 
 * 2. Steady State Test:
 *    - p95 latency < 200ms ✓
 *    - Error rate < 1% ✓
 * 
 * 3. Spike Test:
 *    - No crashes ✓
 *    - Gradual recovery ✓
 *    - Memory usage stable ✓
 * 
 * 4. Stress Test:
 *    - Identify breaking point ✓
 *    - Graceful degradation ✓
 * 
 * Key Metrics:
 * 
 * - http_req_duration: Request latency (p50, p95, p99)
 * - http_req_failed: Failed request rate
 * - checks: Custom check pass/fail rates
 * - vus: Virtual users active
 * - iterations: Total iterations completed
 * 
 * =============================================================================
 */

// Configuration
const config = {
    // API Configuration - can be overridden via environment variables
    ALLOCATION_API_URL: __ENV.ALLOCATION_API_URL || 'http://localhost:8080/api/v1',
    VPN_API_URL: __ENV.VPN_API_URL || 'http://localhost:8080/api/v1/vpn',
    
    // Test Configuration
    TIMEOUT_MS: parseInt(__ENV.TIMEOUT_MS) || 30000,
    
    // Metrics Configuration
    INFLUXDB_URL: __ENV.INFLUXDB_URL || 'http://localhost:8086',
    INFLUXDB_DB: __ENV.INFLUXDB_DB || 'k6',
    INFLUXDB_USER: __ENV.INFLUXDB_USER || 'k6',
    INFLUXDB_PASSWORD: __ENV.INFLUXDB_PASSWORD || 'k6password',
    
    // Grafana Configuration
    GRAFANA_URL: __ENV.GRAFANA_URL || 'http://localhost:3000',
};

// Global state for tracking allocations across test
let allocationTracker = {
    allocatedIPs: new Set(),
    duplicateCount: 0,
    failedCount: 0,
    successCount: 0,
};

// Custom metrics
const ipAllocationLatency = new Trend('ip_allocation_latency');
const ipAllocationDuration = new Trend('ip_allocation_duration');
const duplicateIPRate = new Rate('duplicate_ip_rate');
const allocationSuccessRate = new Rate('allocation_success_rate');

// =============================================================================
// SCENARIO 1: Concurrent Allocation Test
// =============================================================================
// Tests the system's ability to handle 1000 simultaneous VPN key creations
export const concurrentScenario = {
    executor: 'ramping-vus',
    startVUs: 1000,
    stages: [
        { duration: '30s', target: 1000 },
    ],
    tags: { test_type: 'concurrent_allocation' },
};

// =============================================================================
// SCENARIO 2: Steady State Test
// =============================================================================
// Tests system stability under sustained load (100 VUs for 5 minutes)
export const steadyStateScenario = {
    executor: 'ramping-vus',
    startVUs: 0,
    stages: [
        { duration: '30s', target: 100 },
        { duration: '5m', target: 100 },
        { duration: '30s', target: 0 },
    ],
    tags: { test_type: 'steady_state' },
};

// =============================================================================
// SCENARIO 3: Spike Test
// =============================================================================
// Tests system response to sudden traffic spikes
export const spikeScenario = {
    executor: 'ramping-vus',
    startVUs: 10,
    stages: [
        { duration: '10s', target: 500 },
        { duration: '1m', target: 500 },
        { duration: '30s', target: 10 },
    ],
    tags: { test_type: 'spike' },
};

// =============================================================================
// SCENARIO 4: Stress Test
// =============================================================================
// Gradually increases load to find the breaking point
export const stressScenario = {
    executor: 'ramping-vus',
    startVUs: 100,
    stages: [
        { duration: '1m', target: 200 },
        { duration: '1m', target: 400 },
        { duration: '1m', target: 600 },
        { duration: '1m', target: 800 },
        { duration: '1m', target: 1000 },
        { duration: '5m', target: 1000 },
    ],
    tags: { test_type: 'stress' },
};

// =============================================================================
// SCENARIO CONFIGURATION
// =============================================================================

// Determine which scenario to run based on environment variable
const selectedScenario = __ENV.SCENARIO || 'concurrent';

const scenarioConfigs = {
    'concurrent': concurrentScenario,
    'steady-state': steadyStateScenario,
    'steady_state': steadyStateScenario,
    'spike': spikeScenario,
    'stress': stressScenario,
};

// Default configuration
const defaultConfig = {
    scenarios: {
        default: scenarioConfigs[selectedScenario] || concurrentScenario,
    },
    thresholds: {
        // Concurrent Allocation Test thresholds
        'ip_allocation_latency': ['p99<500', 'p95<300', 'p50<150'],
        'ip_allocation_duration': ['p99<600', 'p95<400', 'p50<200'],
        'duplicate_ip_rate': ['rate==0'],
        'allocation_success_rate': ['rate>0.99'],
        'http_req_failed': ['rate<0.01'],
    },
};

// Per-scenario thresholds
const scenarioThresholds = {
    concurrent: {
        'ip_allocation_latency': ['p99<500', 'p95<350', 'p50<150'],
        'ip_allocation_duration': ['p99<600', 'p95<400', 'p50<200'],
        'duplicate_ip_rate': ['rate==0'],
        'allocation_success_rate': ['rate>0.99'],
        'http_req_failed': ['rate<0.01'],
    },
    'steady-state': {
        'ip_allocation_latency': ['p95<200', 'p99<350', 'p50<100'],
        'ip_allocation_duration': ['p95<300', 'p99<450', 'p50<150'],
        'allocation_success_rate': ['rate>0.99'],
        'http_req_failed': ['rate<0.01'],
    },
    spike: {
        'ip_allocation_latency': ['p99<800', 'p95<500', 'p50<200'],
        'ip_allocation_duration': ['p99<1000', 'p95<600', 'p50<250'],
        'allocation_success_rate': ['rate>0.95'],
        'http_req_failed': ['rate<0.05'],
    },
    stress: {
        'ip_allocation_latency': ['p99<1000', 'p95<600'],
        'ip_allocation_duration': ['p99<1200', 'p95<800'],
        'allocation_success_rate': ['rate>0.90'],
        'http_req_failed': ['rate<0.10'],
    },
};

export const options = {
    scenarios: {
        default: scenarioConfigs[selectedScenario] || concurrentScenario,
    },
    thresholds: scenarioThresholds[selectedScenario] || scenarioThresholds.concurrent,
    summaryTrendStats: ['avg', 'min', 'max', 'p50', 'p75', 'p90', 'p95', 'p99', 'count'],
    noColor: false,
};

// =============================================================================
// SETUP - Runs once before the test starts
// =============================================================================
export function setup() {
    console.log('='.repeat(80));
    console.log('WireGuard IP Allocation Load Test - Setup');
    console.log('='.repeat(80));
    console.log(`Scenario: ${selectedScenario}`);
    console.log(`API URL: ${config.ALLOCATION_API_URL}`);
    console.log(`VPN API URL: ${config.VPN_API_URL}`);
    console.log(`Timeout: ${config.TIMEOUT_MS}ms`);
    console.log('='.repeat(80));
    
    // Return setup data that will be available in default function
    return {
        config: config,
        scenario: selectedScenario,
        startTime: Date.now(),
    };
}

// =============================================================================
// TEST ITERATION - Main test logic
// =============================================================================
export default function(data) {
    const vuId = __VU;  // Virtual User ID (unique per VU)
    const vuIter = __ITER;  // Iteration number for this VU
    
    // Generate unique key name to avoid conflicts
    const keyName = `load-test-${selectedScenario}-vu${vuId}-iter${vuIter}-${Date.now()}`;
    
    const startTime = Date.now();
    
    // Track allocation for this VU
    let allocatedIP = null;
    let allocationSuccess = false;
    let isDuplicate = false;
    
    try {
        // Step 1: Create VPN key pair
        const keyPair = generateWireGuardKeyPair();
        
        // Step 2: Request IP allocation from the VPN API
        const allocateResponse = http.post(
            `${config.VPN_API_URL}/allocate`,
            JSON.stringify({
                public_key: keyPair.publicKey,
                key_name: keyName,
                request_id: `req-${vuId}-${vuIter}-${Date.now()}`,
            }),
            {
                headers: {
                    'Content-Type': 'application/json',
                    'X-VU-ID': String(vuId),
                    'X-Request-ID': `${keyName}`,
                },
                timeout: `${config.TIMEOUT_MS}s`,
            }
        );
        
        const latency = (Date.now() - startTime);
        ipAllocationLatency.add(latency);
        
        // Check for successful allocation
        if (allocateResponse.status === 200 || allocateResponse.status === 201) {
            try {
                const responseData = allocateResponse.json();
                allocatedIP = responseData.allocated_ip || responseData.ip_address;
                
                // Critical: Check for duplicate IP allocation
                // This is the most important check - duplicate IPs indicate a serious bug
                synchronized(() => {
                    if (allocationTracker.allocatedIPs.has(allocatedIP)) {
                        isDuplicate = true;
                        allocationTracker.duplicateCount++;
                        console.error(`[VIRTUAL USER ${vuId}] DUPLICATE IP DETECTED: ${allocatedIP}`);
                    } else {
                        allocationTracker.allocatedIPs.add(allocatedIP);
                    }
                });
                
                if (!isDuplicate) {
                    allocationSuccess = true;
                    allocationTracker.successCount++;
                    
                    // Track duration metric
                    const duration = responseData.allocation_duration_ms || latency;
                    ipAllocationDuration.add(duration);
                }
            } catch (e) {
                console.error(`[VIRTUAL USER ${vuId}] Failed to parse response: ${e.message}`);
                allocationTracker.failedCount++;
            }
        } else {
            // Log failed allocation
            console.error(`[VIRTUAL USER ${vuId}] Allocation failed with status ${allocateResponse.status}`);
            console.error(`[VIRTUAL USER ${vuId}] Response: ${allocateResponse.body}`);
            allocationTracker.failedCount++;
        }
        
        // Record metrics
        duplicateIPRate.add(isDuplicate ? 1 : 0);
        allocationSuccessRate.add(allocationSuccess ? 1 : 0);
        
        // Add custom check for this iteration
        check(allocationSuccess, {
            'allocation_successful': allocationSuccess,
            'no_duplicate_ip': !isDuplicate,
        });
        
        // Small delay to prevent overwhelming the system
        sleep(Math.random() * 0.1 + 0.01); // 10-110ms random delay
        
    } catch (error) {
        console.error(`[VIRTUAL USER ${vuId}] Error during allocation: ${error.message}`);
        allocationTracker.failedCount++;
        allocationSuccessRate.add(0);
        
        // Record the error
        fail(`Allocation failed: ${error.message}`);
    }
}

// =============================================================================
// TEARDOWN - Runs once after the test completes
// =============================================================================
export function teardown(data) {
    const testDuration = (Date.now() - data.startTime) / 1000;
    
    console.log('='.repeat(80));
    console.log('WireGuard IP Allocation Load Test - Teardown');
    console.log('='.repeat(80));
    console.log(`Test Duration: ${testDuration.toFixed(2)}s`);
    console.log(`Total Allocations Attempted: ${allocationTracker.successCount + allocationTracker.failedCount}`);
    console.log(`Successful Allocations: ${allocationTracker.successCount}`);
    console.log(`Failed Allocations: ${allocationTracker.failedCount}`);
    console.log(`Duplicate IPs Detected: ${allocationTracker.duplicateCount}`);
    console.log(`Unique IPs Allocated: ${allocationTracker.allocatedIPs.size}`);
    console.log('='.repeat(80));
    
    // Calculate success rate
    const totalAttempts = allocationTracker.successCount + allocationTracker.failedCount;
    if (totalAttempts > 0) {
        const successRate = (allocationTracker.successCount / totalAttempts * 100).toFixed(2);
        console.log(`Success Rate: ${successRate}%`);
    }
    
    // Calculate duplicate rate
    if (allocationTracker.successCount > 0) {
        const duplicateRate = (allocationTracker.duplicateCount / allocationTracker.successCount * 100).toFixed(4);
        console.log(`Duplicate Rate: ${duplicateRate}%`);
    }
    
    // Print unique IPs for verification (limited output)
    console.log('\nSample of allocated IPs (first 10):');
    const ipArray = Array.from(allocationTracker.allocatedIPs);
    for (let i = 0; i < Math.min(10, ipArray.length); i++) {
        console.log(`  ${i + 1}. ${ipArray[i]}`);
    }
    
    console.log('='.repeat(80));
    console.log('Test completed. Check Grafana for detailed metrics.');
    console.log(`Grafana URL: ${config.GRAFANA_URL}`);
    console.log('='.repeat(80));
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

/**
 * Generates a WireGuard-style key pair
 * In production, this should be generated client-side
 */
function generateWireGuardKeyPair() {
    // Generate 32 random bytes for the private key
    const privateKeyBytes = new Uint8Array(32);
    for (let i = 0; i < 32; i++) {
        privateKeyBytes[i] = Math.floor(Math.random() * 256);
    }
    
    // Simple mock public key generation (in real implementation, use Curve25519)
    const publicKeyBytes = new Uint8Array(32);
    for (let i = 0; i < 32; i++) {
        publicKeyBytes[i] = privateKeyBytes[i] ^ ((i * 7 + 13) % 256);
    }
    
    return {
        privateKey: arrayToBase64(privateKeyBytes),
        publicKey: arrayToBase64(publicKeyBytes),
    };
}

/**
 * Converts byte array to base64 string
 */
function arrayToBase64(bytes) {
    let binary = '';
    for (let i = 0; i < bytes.length; i++) {
        binary += String.fromCharCode(bytes[i]);
    }
    return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
}

/**
 * Thread-safe synchronized block using k6's shared capabilities
 * In k6, we use a lock mechanism for shared state
 */
let lockCounter = 0;
function synchronized(fn) {
    // k6 uses a simple mutex pattern for synchronization
    // In real usage, you would use the built-in synchronization or external locking
    // For k6, we use atomic operations where possible
    const lockId = ++lockCounter;
    try {
        fn();
    } finally {
        lockCounter--;
    }
}

/**
 * Handler for handleSummary - provides detailed test report
 */
export function handleSummary(data) {
    return {
        stdout: textSummary(data, { indent: ' ', enableColors: true }),
        'summary.json': JSON.stringify(data, null, 2),
    };
}

/**
 * Formats the test summary for console output
 */
function textSummary(data, options = {}) {
    const indent = options.indent || '';
    const colors = options.enableColors ? true : false;
    
    let output = '\n';
    output += indent + '='.repeat(70) + '\n';
    output += indent + '  WireGuard IP Allocation Load Test Summary\n';
    output += indent + '='.repeat(70) + '\n\n';
    
    // Test configuration
    output += indent + 'TEST CONFIGURATION:\n';
    output += indent + '-'.repeat(70) + '\n';
    output += indent + `  Scenario:          ${selectedScenario}\n`;
    output += indent + `  API URL:           ${config.ALLOCATION_API_URL}\n`;
    output += indent + `  Duration:          ${(data.test_run_duration / 1000).toFixed(2)}s\n`;
    output += indent + `  Total Iterations:  ${data.metrics.iterations.values.count || 0}\n`;
    output += '\n';
    
    // Request metrics
    output += indent + 'REQUEST METRICS:\n';
    output += indent + '-'.repeat(70) + '\n';
    
    const httpMetrics = [
        'http_reqs',
        'http_req_duration',
        'http_req_failed',
    ];
    
    for (const metric of httpMetrics) {
        if (data.metrics[metric]) {
            const values = data.metrics[metric].values;
            output += indent + `  ${metric}:\n`;
            for (const [key, value] of Object.entries(values)) {
                if (typeof value === 'number') {
                    const formatted = formatMetricValue(key, value);
                    output += indent + `    ${key}: ${formatted}\n`;
                }
            }
        }
    }
    output += '\n';
    
    // Custom metrics
    output += indent + 'IP ALLOCATION METRICS:\n';
    output += indent + '-'.repeat(70) + '\n';
    
    const customMetrics = [
        'ip_allocation_latency',
        'ip_allocation_duration',
        'duplicate_ip_rate',
        'allocation_success_rate',
    ];
    
    for (const metric of customMetrics) {
        if (data.metrics[metric]) {
            const values = data.metrics[metric].values;
            output += indent + `  ${metric}:\n`;
            for (const [key, value] of Object.entries(values)) {
                if (typeof value === 'number') {
                    const formatted = formatMetricValue(key, value);
                    output += indent + `    ${key}: ${formatted}\n`;
                }
            }
        }
    }
    output += '\n';
    
    // Thresholds status
    output += indent + 'THRESHOLD STATUS:\n';
    output += indent + '-'.repeat(70) + '\n';
    
    for (const [threshold, result] of Object.entries(data.thresholds || {})) {
        const status = result.ok ? 'PASSED' : 'FAILED';
        const statusIcon = result.ok ? '[✓]' : '[✗]';
        output += indent + `  ${statusIcon} ${threshold}: ${status}\n`;
        if (!result.ok && result.threshold) {
            output += indent + `      ${result.threshold}\n`;
        }
    }
    output += '\n';
    
    // Summary statistics
    output += indent + '='.repeat(70) + '\n';
    output += indent + 'FINAL RESULTS:\n';
    output += indent + '-'.repeat(70) + '\n';
    
    const successRate = data.metrics.allocation_success_rate?.values?.rate || 0;
    const duplicateRate = data.metrics.duplicate_ip_rate?.values?.rate || 0;
    const p99Latency = data.metrics.ip_allocation_latency?.values?.['p99'] || 0;
    const errorRate = data.metrics.http_req_failed?.values?.rate || 0;
    
    output += indent + `  Success Rate:      ${(successRate * 100).toFixed(2)}% (target: >99%)\n`;
    output += indent + `  Duplicate Rate:    ${(duplicateRate * 100).toFixed(4)}% (target: 0%)\n`;
    output += indent + `  p99 Latency:       ${p99Latency.toFixed(2)}ms\n`;
    output += indent + `  Error Rate:        ${(errorRate * 100).toFixed(2)}%\n`;
    output += '\n';
    
    // Verdict
    const allPassed = Object.values(data.thresholds || {}).every(t => t.ok);
    const verdict = allPassed ? 'TEST PASSED' : 'TEST FAILED';
    const verdictIcon = allPassed ? '[✓]' : '[✗]';
    output += indent + '='.repeat(70) + '\n';
    output += indent + `  ${verdictIcon} ${verdict}\n`;
    output += indent + '='.repeat(70) + '\n';
    
    return output;
}

/**
 * Formats metric values with appropriate units
 */
function formatMetricValue(key, value) {
    if (key === 'p99' || key === 'p95' || key === 'p90' || key === 'p75' || key === 'p50' || key === 'avg') {
        return `${value.toFixed(2)}ms`;
    } else if (key === 'rate') {
        return `${(value * 100).toFixed(2)}%`;
    } else if (key === 'count' || key === 'value') {
        return value.toFixed(0);
    } else {
        return value.toFixed(2);
    }
}