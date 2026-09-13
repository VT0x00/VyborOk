#!/usr/bin/env bash
set -e

BASE=http://localhost:8080
SUFFIX=$(date +%s)
EMAIL="test_${SUFFIX}@example.com"
USERNAME="user_${SUFFIX}"
PASSWORD="secret12345"

echo "== 1. Register =="
REG=$(curl -s -X POST "$BASE/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"username\":\"$USERNAME\"}")
echo "$REG" | jq

echo "== 2. Login =="
LOGIN=$(curl -s -X POST "$BASE/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")
echo "$LOGIN" | jq

ACCESS=$(echo "$LOGIN" | jq -r '.access_token')
REFRESH=$(echo "$LOGIN" | jq -r '.refresh_token')

echo "== 3. GetMe =="
curl -s "$BASE/auth/me" -H "Authorization: Bearer $ACCESS" | jq

echo "== 4. Refresh =="
NEW=$(curl -s -X POST "$BASE/auth/refresh" \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\":\"$REFRESH\"}")
echo "$NEW" | jq

NEW_ACCESS=$(echo "$NEW" | jq -r '.access_token')

echo "== 5. GetMe with new access =="
curl -s "$BASE/auth/me" -H "Authorization: Bearer $NEW_ACCESS" | jq

echo "== Done =="
