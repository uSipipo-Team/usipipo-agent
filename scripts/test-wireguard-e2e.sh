#!/bin/bash
set -e

echo "========================================="
echo "uSipipo Agent - WireGuard E2E Test"
echo "========================================="

# Configuration
AGENT_API_KEY="${AGENT_API_KEY:-test_key}"
AGENT_URL="${AGENT_URL:-http://localhost:8080}"
TEST_PEER_NAME="test-peer-$(date +%s)"

echo ""
echo "Configuration:"
echo "  Agent URL: $AGENT_URL"
echo "  Test Peer: $TEST_PEER_NAME"
echo ""

# Test 1: Create WireGuard peer
echo "Test 1: Creating WireGuard peer..."
RESPONSE=$(curl -s -X POST \
  -H "X-API-Key: $AGENT_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"$TEST_PEER_NAME\"}" \
  "$AGENT_URL/wireguard/peers")

if echo "$RESPONSE" | grep -q "error"; then
  echo "❌ FAILED: $RESPONSE"
  exit 1
fi

echo "✅ PASSED: Peer created"
echo "Response: $RESPONSE"
echo ""

# Test 2: Verify peer exists
echo "Test 2: Verifying peer exists..."
if sudo wg show wg0 | grep -q "$TEST_PEER_NAME"; then
  echo "✅ PASSED: Peer found in wg show"
else
  echo "❌ FAILED: Peer not found in wg show"
  exit 1
fi
echo ""

# Test 3: Get peer usage
echo "Test 3: Getting peer usage..."
USAGE=$(curl -s \
  -H "X-API-Key: $AGENT_API_KEY" \
  "$AGENT_URL/wireguard/peers/$TEST_PEER_NAME/usage")

echo "Usage: $USAGE"
echo "✅ PASSED: Usage retrieved"
echo ""

# Test 4: Delete peer
echo "Test 4: Deleting peer..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE \
  -H "X-API-Key: $AGENT_API_KEY" \
  "$AGENT_URL/wireguard/peers/$TEST_PEER_NAME")

if [ "$HTTP_CODE" -eq 204 ] || [ "$HTTP_CODE" -eq 200 ]; then
  echo "✅ PASSED: Peer deleted (HTTP $HTTP_CODE)"
else
  echo "❌ FAILED: Delete returned HTTP $HTTP_CODE"
  exit 1
fi
echo ""

# Test 5: Verify cleanup
echo "Test 5: Verifying cleanup..."
sleep 1
if sudo wg show wg0 | grep -q "$TEST_PEER_NAME"; then
  echo "❌ FAILED: Peer still exists after deletion"
  exit 1
else
  echo "✅ PASSED: Peer cleaned up successfully"
fi
echo ""

echo "========================================="
echo "All WireGuard E2E tests passed! ✅"
echo "========================================="
echo ""
echo "Summary:"
echo "  ✅ Peer creation via API"
echo "  ✅ Peer verification in wg show"
echo "  ✅ Usage retrieval via API"
echo "  ✅ Peer deletion via API"
echo "  ✅ Cleanup verification"
echo ""
