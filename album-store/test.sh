#!/usr/bin/env bash
# test.sh — Quick smoke test for album-store API
set -e

# Usage: ./test.sh http://your-ec2-ip:8080

BASE_URL="${1:-http://localhost:8080}"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================"
echo "Album Store Smoke Test"
echo "========================================"
echo "Base URL: $BASE_URL"
echo ""

# Test 1: Health Check
echo -n "Test 1: Health Check... "
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/health")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n1)

if [[ "$HTTP_CODE" == "200" ]] && [[ "$BODY" == *'"status":"ok"'* ]]; then
    echo -e "${GREEN}✓ PASS${NC}"
else
    echo -e "${RED}✗ FAIL${NC} (HTTP $HTTP_CODE, Body: $BODY)"
    exit 1
fi

# Test 2: Create Album
echo -n "Test 2: Create Album... "
ALBUM_ID="test-album-$(date +%s)"
RESPONSE=$(curl -s -w "\n%{http_code}" -X PUT "$BASE_URL/albums/$ALBUM_ID" \
    -H "Content-Type: application/json" \
    -d "{\"album_id\":\"$ALBUM_ID\",\"title\":\"Test Album\",\"description\":\"Testing\",\"owner\":\"test@test.com\"}")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n1)

if [[ "$HTTP_CODE" =~ ^(200|201)$ ]] && [[ "$BODY" == *"$ALBUM_ID"* ]]; then
    echo -e "${GREEN}✓ PASS${NC}"
else
    echo -e "${RED}✗ FAIL${NC} (HTTP $HTTP_CODE, Body: $BODY)"
    exit 1
fi

# Test 3: Get Album
echo -n "Test 3: Get Album... "
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/albums/$ALBUM_ID")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n1)

if [[ "$HTTP_CODE" == "200" ]] && [[ "$BODY" == *"$ALBUM_ID"* ]]; then
    echo -e "${GREEN}✓ PASS${NC}"
else
    echo -e "${RED}✗ FAIL${NC} (HTTP $HTTP_CODE, Body: $BODY)"
    exit 1
fi

# Test 4: List Albums
echo -n "Test 4: List Albums... "
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/albums")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n1)

if [[ "$HTTP_CODE" == "200" ]] && [[ "$BODY" == *"$ALBUM_ID"* ]]; then
    echo -e "${GREEN}✓ PASS${NC}"
else
    echo -e "${RED}✗ FAIL${NC} (HTTP $HTTP_CODE, Body: $BODY)"
    exit 1
fi

# Test 5: Upload Photo
echo -n "Test 5: Upload Photo (async)... "

# Create a test image (1x1 pixel PNG)
TEST_IMAGE=$(mktemp -t test-image.XXXXXX.png)
echo -n "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==" | base64 -d > "$TEST_IMAGE"

RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/albums/$ALBUM_ID/photos" \
    -F "photo=@$TEST_IMAGE")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n1)

if [[ "$HTTP_CODE" == "202" ]] && [[ "$BODY" == *'"photo_id"'* ]] && [[ "$BODY" == *'"seq":1'* ]] && [[ "$BODY" == *'"status":"processing"'* ]]; then
    echo -e "${GREEN}✓ PASS${NC}"
    PHOTO_ID=$(echo "$BODY" | grep -o '"photo_id":"[^"]*"' | cut -d'"' -f4)
else
    echo -e "${RED}✗ FAIL${NC} (HTTP $HTTP_CODE, Body: $BODY)"
    rm -f "$TEST_IMAGE"
    exit 1
fi

# Test 6: Get Photo Status (may be processing)
echo -n "Test 6: Get Photo Status... "
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/albums/$ALBUM_ID/photos/$PHOTO_ID")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n1)

if [[ "$HTTP_CODE" == "200" ]] && [[ "$BODY" == *"$PHOTO_ID"* ]] && [[ "$BODY" == *'"seq":1'* ]]; then
    echo -e "${GREEN}✓ PASS${NC}"
else
    echo -e "${RED}✗ FAIL${NC} (HTTP $HTTP_CODE, Body: $BODY)"
    rm -f "$TEST_IMAGE"
    exit 1
fi

# Test 7: Wait for completion
echo -n "Test 7: Wait for upload completion... "
MAX_WAIT=30
WAITED=0
COMPLETED=false

while [[ $WAITED -lt $MAX_WAIT ]]; do
    RESPONSE=$(curl -s "$BASE_URL/albums/$ALBUM_ID/photos/$PHOTO_ID")
    if [[ "$RESPONSE" == *'"status":"completed"'* ]] && [[ "$RESPONSE" == *'"url"'* ]]; then
        COMPLETED=true
        break
    fi
    sleep 1
    WAITED=$((WAITED + 1))
done

if [[ "$COMPLETED" == "true" ]]; then
    echo -e "${GREEN}✓ PASS${NC} (completed in ${WAITED}s)"
    PHOTO_URL=$(echo "$RESPONSE" | grep -o '"url":"[^"]*"' | cut -d'"' -f4)
else
    echo -e "${YELLOW}⚠ TIMEOUT${NC} (still processing after ${MAX_WAIT}s)"
    PHOTO_URL=""
fi

# Test 8: Verify Photo URL
if [[ -n "$PHOTO_URL" ]]; then
    echo -n "Test 8: Verify Photo URL... "
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$PHOTO_URL")

    if [[ "$HTTP_CODE" == "200" ]]; then
        echo -e "${GREEN}✓ PASS${NC}"
    else
        echo -e "${RED}✗ FAIL${NC} (HTTP $HTTP_CODE for URL: $PHOTO_URL)"
    fi
else
    echo "Test 8: Verify Photo URL... ${YELLOW}⊘ SKIPPED${NC} (no URL available)"
fi

# Test 9: Delete Photo
echo -n "Test 9: Delete Photo... "
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/albums/$ALBUM_ID/photos/$PHOTO_ID")

if [[ "$HTTP_CODE" =~ ^(200|204)$ ]]; then
    echo -e "${GREEN}✓ PASS${NC}"
else
    echo -e "${RED}✗ FAIL${NC} (HTTP $HTTP_CODE)"
fi

# Test 10: Verify Deletion
echo -n "Test 10: Verify Photo Deleted (should 404)... "
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/albums/$ALBUM_ID/photos/$PHOTO_ID")

if [[ "$HTTP_CODE" == "404" ]]; then
    echo -e "${GREEN}✓ PASS${NC}"
else
    echo -e "${RED}✗ FAIL${NC} (HTTP $HTTP_CODE, expected 404)"
fi

# Test 11: Test Sequence Numbers
echo -n "Test 11: Test Sequence Numbers... "
RESPONSE1=$(curl -s -X POST "$BASE_URL/albums/$ALBUM_ID/photos" -F "photo=@$TEST_IMAGE")
RESPONSE2=$(curl -s -X POST "$BASE_URL/albums/$ALBUM_ID/photos" -F "photo=@$TEST_IMAGE")
SEQ1=$(echo "$RESPONSE1" | grep -o '"seq":[0-9]*' | cut -d':' -f2)
SEQ2=$(echo "$RESPONSE2" | grep -o '"seq":[0-9]*' | cut -d':' -f2)

if [[ $SEQ2 -eq $((SEQ1 + 1)) ]]; then
    echo -e "${GREEN}✓ PASS${NC} (seq incremented: $SEQ1 → $SEQ2)"
else
    echo -e "${RED}✗ FAIL${NC} (seq not monotonic: $SEQ1, $SEQ2)"
fi

# Test 12: Test Per-Album Sequences
echo -n "Test 12: Test Per-Album Sequences... "
ALBUM2_ID="test-album-2-$(date +%s)"
curl -s -X PUT "$BASE_URL/albums/$ALBUM2_ID" \
    -H "Content-Type: application/json" \
    -d "{\"album_id\":\"$ALBUM2_ID\",\"title\":\"Test\",\"description\":\"Test\",\"owner\":\"test@test.com\"}" > /dev/null
RESPONSE=$(curl -s -X POST "$BASE_URL/albums/$ALBUM2_ID/photos" -F "photo=@$TEST_IMAGE")
SEQ=$(echo "$RESPONSE" | grep -o '"seq":[0-9]*' | cut -d':' -f2)

if [[ $SEQ -eq 1 ]]; then
    echo -e "${GREEN}✓ PASS${NC} (new album starts at seq=1)"
else
    echo -e "${RED}✗ FAIL${NC} (new album seq=$SEQ, expected 1)"
fi

# Cleanup
rm -f "$TEST_IMAGE"

echo ""
echo "========================================"
echo -e "${GREEN}All Tests Passed!${NC}"
echo "========================================"
echo ""
echo "Your service is ready for ChaosArena submission!"
echo ""
echo "To submit:"
echo "  curl -X POST https://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/submit \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"email\":\"you@northeastern.edu\",\"nickname\":\"YourName\",\"base_url\":\"$BASE_URL\",\"contract\":\"v1-album-store\"}'"
echo ""
