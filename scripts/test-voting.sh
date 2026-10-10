#!/usr/bin/env bash
# scripts/test-voting.sh
#
# E2E verification for Phase 3 (Voting). Covers spec 0028 R1–R29.
#
# Requires:
#   - backend running on $API (make run)
#   - PostgreSQL container up
#   - curl, jq, docker
#
# Usage:
#   ./scripts/test-voting.sh
#   API=http://localhost:9000 PG_CONTAINER=my-pg ./scripts/test-voting.sh

set -uo pipefail

API=${API:-http://localhost:8080}
PG_CONTAINER=${PG_CONTAINER:-vyborok-postgres}
PG_USER=${PG_USER:-postgres}
PG_DB=${PG_DB:-vyborok}

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
CYAN=$'\033[0;36m'
YELLOW=$'\033[0;33m'
NC=$'\033[0m'

PASS=0
FAIL=0
SUFFIX=$(date +%s)

# ---------- helpers ----------

pass()    { echo "  ${GREEN}✓${NC} $1"; PASS=$((PASS+1)); }
fail()    { echo "  ${RED}✗${NC} $1"; FAIL=$((FAIL+1)); }
section() { echo; echo "${CYAN}### $1${NC}"; }

# do_req METHOD PATH [DATA] [TOKEN]
# Sets global HTTP_CODE and BODY.
do_req() {
  local method="$1" path="$2" data="${3:-}" token="${4:-}"
  local args=(-s -X "$method" "$API$path" -w $'\n%{http_code}')
  [ -n "$data" ]  && args+=(-H 'Content-Type: application/json' -d "$data")
  [ -n "$token" ] && args+=(-H "Authorization: Bearer $token")
  local raw
  raw=$(curl "${args[@]}")
  HTTP_CODE=$(printf '%s' "$raw" | tail -n1)
  BODY=$(printf '%s' "$raw" | sed '$d')
}

expect_code() {
  local desc="$1" expected="$2"
  if [ "$HTTP_CODE" = "$expected" ]; then
    pass "$desc (HTTP $HTTP_CODE)"
  else
    fail "$desc — expected HTTP $expected, got $HTTP_CODE; body: $BODY"
  fi
}

expect_eq() {
  local desc="$1" expected="$2" actual="$3"
  if [ "$expected" = "$actual" ]; then
    pass "$desc"
  else
    fail "$desc — expected '$expected', got '$actual'"
  fi
}

# expect_jq DESC JQ_EXPR — succeeds if expr yields "true"
expect_jq() {
  local desc="$1" expr="$2"
  local out
  out=$(printf '%s' "$BODY" | jq -r "$expr" 2>/dev/null)
  if [ "$out" = "true" ]; then pass "$desc"; else fail "$desc — jq '$expr' yielded '$out'; body: $BODY"; fi
}

psql_t() {
  docker exec -i "$PG_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -tA -c "$1" 2>/dev/null | tr -d '\r'
}

# ---------- preflight ----------

if ! curl -sf "$API/health" >/dev/null; then
  echo "${RED}FATAL: server not reachable at $API${NC}"
  exit 1
fi
if ! docker exec "$PG_CONTAINER" true 2>/dev/null; then
  echo "${RED}FATAL: cannot exec into container '$PG_CONTAINER'${NC}"
  exit 1
fi
if ! command -v jq >/dev/null; then
  echo "${RED}FATAL: jq is required${NC}"
  exit 1
fi

# ---------- setup ----------

section "Setup"

do_req POST /auth/register \
  "{\"email\":\"v_owner_$SUFFIX@example.com\",\"password\":\"secret12345\",\"username\":\"v_owner_$SUFFIX\"}"
OWNER_TOKEN=$(printf '%s' "$BODY" | jq -r .access_token)
OWNER_ID=$(printf '%s' "$BODY" | jq -r .profile.id)

do_req POST /auth/register \
  "{\"email\":\"v_alice_$SUFFIX@example.com\",\"password\":\"secret12345\",\"username\":\"v_alice_$SUFFIX\"}"
ALICE_TOKEN=$(printf '%s' "$BODY" | jq -r .access_token)
ALICE_ID=$(printf '%s' "$BODY" | jq -r .profile.id)

if [ -z "$OWNER_TOKEN" ] || [ "$OWNER_TOKEN" = "null" ] || [ -z "$ALICE_TOKEN" ] || [ "$ALICE_TOKEN" = "null" ]; then
  echo "${RED}FATAL: failed to register test users${NC}"
  echo "OWNER body: $BODY"
  exit 1
fi
echo "  owner_id=$OWNER_ID"
echo "  alice_id=$ALICE_ID"

# P_NONANON: single + text
do_req POST /polls '{"title":"nonanon","questions":[
  {"type":"single","title":"Q1","options":[{"text":"A"},{"text":"B"}]},
  {"type":"text","title":"Q2","text_max_length":100}
]}' "$OWNER_TOKEN"
NONANON_ID=$(printf '%s' "$BODY" | jq -r .poll.id)
NONANON_Q1=$(printf '%s' "$BODY" | jq -r '.poll.questions[0].id')
NONANON_Q1_O1=$(printf '%s' "$BODY" | jq -r '.poll.questions[0].options[0].id')
NONANON_Q2=$(printf '%s' "$BODY" | jq -r '.poll.questions[1].id')

# P_ANON: single + multiple + scale + text
do_req POST /polls '{"title":"anon","anonymous":true,"questions":[
  {"type":"single","title":"Q1","options":[{"text":"X"},{"text":"Y"},{"text":"Z"}]},
  {"type":"multiple","title":"Q2","options":[{"text":"M"},{"text":"N"},{"text":"P"}]},
  {"type":"scale","title":"Q3","scale_min":1,"scale_max":5},
  {"type":"text","title":"Q4","text_max_length":50}
]}' "$OWNER_TOKEN"
ANON_ID=$(printf '%s' "$BODY" | jq -r .poll.id)
ANON_Q1=$(printf '%s' "$BODY" | jq -r '.poll.questions[0].id')
ANON_Q1_O1=$(printf '%s' "$BODY" | jq -r '.poll.questions[0].options[0].id')
ANON_Q1_O2=$(printf '%s' "$BODY" | jq -r '.poll.questions[0].options[1].id')
ANON_Q2=$(printf '%s' "$BODY" | jq -r '.poll.questions[1].id')
ANON_Q2_O1=$(printf '%s' "$BODY" | jq -r '.poll.questions[1].options[0].id')
ANON_Q2_O2=$(printf '%s' "$BODY" | jq -r '.poll.questions[1].options[1].id')
ANON_Q3=$(printf '%s' "$BODY" | jq -r '.poll.questions[2].id')
ANON_Q4=$(printf '%s' "$BODY" | jq -r '.poll.questions[3].id')

# P_ANON_B: single text, will be closed for R3
do_req POST /polls '{"title":"anon-closed","anonymous":true,"questions":[
  {"type":"text","title":"Q1","text_max_length":50}
]}' "$OWNER_TOKEN"
ANON_B_ID=$(printf '%s' "$BODY" | jq -r .poll.id)
ANON_B_Q1=$(printf '%s' "$BODY" | jq -r '.poll.questions[0].id')

# Valid answers for non-anonymous poll
VALID_NONANON="{\"answers\":[
  {\"question_id\":\"$NONANON_Q1\",\"option_ids\":[\"$NONANON_Q1_O1\"]},
  {\"question_id\":\"$NONANON_Q2\",\"text\":\"hello\"}
]}"

# Valid answers for anonymous poll (all 4 questions)
VALID_ANON="{\"answers\":[
  {\"question_id\":\"$ANON_Q1\",\"option_ids\":[\"$ANON_Q1_O1\"]},
  {\"question_id\":\"$ANON_Q2\",\"option_ids\":[\"$ANON_Q2_O1\",\"$ANON_Q2_O2\"]},
  {\"question_id\":\"$ANON_Q3\",\"value\":3,\"value_set\":true},
  {\"question_id\":\"$ANON_Q4\",\"text\":\"ok\"}
]}"

echo "  nonanon=$NONANON_ID"
echo "  anon=$ANON_ID"
echo "  anon_b=$ANON_B_ID"

# ---------- R1–R4: SubmitVote entry validation ----------

section "R1–R4: SubmitVote entry validation"

# R1
do_req POST /polls/not-a-uuid/vote "$VALID_ANON"
expect_code "R1: bad UUID" 400

# R2
RANDOM_UUID=$(cat /proc/sys/kernel/random/uuid)
do_req POST "/polls/$RANDOM_UUID/vote" "$VALID_ANON"
expect_code "R2: poll does not exist" 404

# R3 — close P_ANON_B via API (Day 27) then vote
do_req POST "/polls/$ANON_B_ID/close" "" "$OWNER_TOKEN"
expect_code "R3 setup: close poll" 200

VALID_ANON_B="{\"answers\":[{\"question_id\":\"$ANON_B_Q1\",\"text\":\"hi\"}]}"
do_req POST "/polls/$ANON_B_ID/vote" "$VALID_ANON_B"
expect_code "R3: closed poll" 409

# R4 — non-anonymous, no JWT
do_req POST "/polls/$NONANON_ID/vote" "$VALID_NONANON"
expect_code "R4: non-anonymous, no JWT" 401

# ---------- R5, R8: non-anonymous happy path + repeat ----------

section "R5, R8: non-anonymous — first vote, then 409"

do_req POST "/polls/$NONANON_ID/vote" "$VALID_NONANON" "$OWNER_TOKEN"
expect_code "R5: owner votes on non-anon" 200

ROW_UID=$(psql_t "SELECT user_id FROM votes WHERE poll_id = '$NONANON_ID' ORDER BY created_at DESC LIMIT 1;")
expect_eq "R5: stored vote.user_id == owner" "$OWNER_ID" "$ROW_UID"

# R8 — second vote from same user
do_req POST "/polls/$NONANON_ID/vote" "$VALID_NONANON" "$OWNER_TOKEN"
expect_code "R8: repeat on non-anon → 409" 409

# ---------- R6, R9a, R7, R9b: anonymous poll ----------

section "R6, R9a: anonymous poll, no JWT"

do_req POST "/polls/$ANON_ID/vote" "$VALID_ANON"
expect_code "R6: anon vote (no JWT)" 200

# R6 check: stored vote must have NULL user_id.
# Pick the vote with NULL user_id — R5/R8 was on a different poll, so anon poll
# has only NULL-user_id votes at this point.
NULL_COUNT=$(psql_t "SELECT COUNT(*) FROM votes WHERE poll_id = '$ANON_ID' AND user_id IS NULL;")
expect_eq "R6: stored vote.user_id IS NULL" "1" "$NULL_COUNT"

# R9a — same anonymous caller votes again; should succeed
do_req POST "/polls/$ANON_ID/vote" "$VALID_ANON"
expect_code "R9a: repeat anon vote (no JWT) → 200" 200

NULL_COUNT_2=$(psql_t "SELECT COUNT(*) FROM votes WHERE poll_id = '$ANON_ID' AND user_id IS NULL;")
expect_eq "R9a: two anonymous votes stored" "2" "$NULL_COUNT_2"

section "R7, R9b: anonymous poll, JWT present"

do_req POST "/polls/$ANON_ID/vote" "$VALID_ANON" "$ALICE_TOKEN"
expect_code "R7: alice votes on anon poll (JWT present)" 200

ALICE_UID=$(psql_t "SELECT user_id FROM votes WHERE poll_id = '$ANON_ID' AND user_id IS NOT NULL ORDER BY created_at DESC LIMIT 1;")
expect_eq "R7: stored vote.user_id == alice" "$ALICE_ID" "$ALICE_UID"

# R9b — alice votes again with the same JWT
do_req POST "/polls/$ANON_ID/vote" "$VALID_ANON" "$ALICE_TOKEN"
expect_code "R9b: repeat by identified voter → 409" 409

# ---------- R10–R23: answer validation ----------

section "R10–R23: answer validation"

# Helper: build answers JSON with a specific mutation on Q1.
# All other answers stay valid.
answer_with_q1() {
  local q1_payload="$1"
  printf '{"answers":[%s,{"question_id":"%s","option_ids":["%s","%s"]},{"question_id":"%s","value":3,"value_set":true},{"question_id":"%s","text":"ok"}]}' \
    "$q1_payload" "$ANON_Q2" "$ANON_Q2_O1" "$ANON_Q2_O2" "$ANON_Q3" "$ANON_Q4"
}

# R10 — single with 0 options
do_req POST "/polls/$ANON_ID/vote" \
  "$(answer_with_q1 "{\"question_id\":\"$ANON_Q1\",\"option_ids\":[]}")"
expect_code "R10: single with 0 options" 400

# R11 — single with 2 options
do_req POST "/polls/$ANON_ID/vote" \
  "$(answer_with_q1 "{\"question_id\":\"$ANON_Q1\",\"option_ids\":[\"$ANON_Q1_O1\",\"$ANON_Q1_O2\"]}")"
expect_code "R11: single with 2 options" 400

# R12 — option does not belong to question (use Q2's option on Q1)
do_req POST "/polls/$ANON_ID/vote" \
  "$(answer_with_q1 "{\"question_id\":\"$ANON_Q1\",\"option_ids\":[\"$ANON_Q2_O1\"]}")"
expect_code "R12: option belongs to another question" 400

# R13 — multiple with 0 options
do_req POST "/polls/$ANON_ID/vote" \
  "{\"answers\":[
    {\"question_id\":\"$ANON_Q1\",\"option_ids\":[\"$ANON_Q1_O1\"]},
    {\"question_id\":\"$ANON_Q2\",\"option_ids\":[]},
    {\"question_id\":\"$ANON_Q3\",\"value\":3,\"value_set\":true},
    {\"question_id\":\"$ANON_Q4\",\"text\":\"ok\"}
  ]}"
expect_code "R13: multiple with 0 options" 400

# R14 — multiple with duplicate option
do_req POST "/polls/$ANON_ID/vote" \
  "{\"answers\":[
    {\"question_id\":\"$ANON_Q1\",\"option_ids\":[\"$ANON_Q1_O1\"]},
    {\"question_id\":\"$ANON_Q2\",\"option_ids\":[\"$ANON_Q2_O1\",\"$ANON_Q2_O1\"]},
    {\"question_id\":\"$ANON_Q3\",\"value\":3,\"value_set\":true},
    {\"question_id\":\"$ANON_Q4\",\"text\":\"ok\"}
  ]}"
expect_code "R14: duplicate option in multiple" 400

# R15 — scale out of range (999)
do_req POST "/polls/$ANON_ID/vote" \
  "{\"answers\":[
    {\"question_id\":\"$ANON_Q1\",\"option_ids\":[\"$ANON_Q1_O1\"]},
    {\"question_id\":\"$ANON_Q2\",\"option_ids\":[\"$ANON_Q2_O1\"]},
    {\"question_id\":\"$ANON_Q3\",\"value\":999,\"value_set\":true},
    {\"question_id\":\"$ANON_Q4\",\"text\":\"ok\"}
  ]}"
expect_code "R15: scale out of range" 400

# R16 — valid scale: implicitly covered by VALID_ANON (R6, R7).

# R17 — text over limit (51 chars on max 50)
LONG_TEXT=$(printf 'a%.0s' $(seq 1 51))
do_req POST "/polls/$ANON_ID/vote" \
  "{\"answers\":[
    {\"question_id\":\"$ANON_Q1\",\"option_ids\":[\"$ANON_Q1_O1\"]},
    {\"question_id\":\"$ANON_Q2\",\"option_ids\":[\"$ANON_Q2_O1\"]},
    {\"question_id\":\"$ANON_Q3\",\"value\":3,\"value_set\":true},
    {\"question_id\":\"$ANON_Q4\",\"text\":\"$LONG_TEXT\"}
  ]}"
expect_code "R17: text over max length" 400

# R18 — text empty
do_req POST "/polls/$ANON_ID/vote" \
  "{\"answers\":[
    {\"question_id\":\"$ANON_Q1\",\"option_ids\":[\"$ANON_Q1_O1\"]},
    {\"question_id\":\"$ANON_Q2\",\"option_ids\":[\"$ANON_Q2_O1\"]},
    {\"question_id\":\"$ANON_Q3\",\"value\":3,\"value_set\":true},
    {\"question_id\":\"$ANON_Q4\",\"text\":\"\"}
  ]}"
expect_code "R18: text empty" 400

# R19 — valid text: implicitly covered by VALID_ANON (R6, R7).

# R20 — missing Q4
do_req POST "/polls/$ANON_ID/vote" \
  "{\"answers\":[
    {\"question_id\":\"$ANON_Q1\",\"option_ids\":[\"$ANON_Q1_O1\"]},
    {\"question_id\":\"$ANON_Q2\",\"option_ids\":[\"$ANON_Q2_O1\"]},
    {\"question_id\":\"$ANON_Q3\",\"value\":3,\"value_set\":true}
  ]}"
expect_code "R20: missing answer for one question" 400

# R21 — Q1 appears twice
do_req POST "/polls/$ANON_ID/vote" \
  "{\"answers\":[
    {\"question_id\":\"$ANON_Q1\",\"option_ids\":[\"$ANON_Q1_O1\"]},
    {\"question_id\":\"$ANON_Q1\",\"option_ids\":[\"$ANON_Q1_O2\"]},
    {\"question_id\":\"$ANON_Q2\",\"option_ids\":[\"$ANON_Q2_O1\"]},
    {\"question_id\":\"$ANON_Q3\",\"value\":3,\"value_set\":true},
    {\"question_id\":\"$ANON_Q4\",\"text\":\"ok\"}
  ]}"
expect_code "R21: duplicate question" 400

# R22 — question_id not part of the poll (from another poll)
do_req POST "/polls/$ANON_ID/vote" \
  "{\"answers\":[
    {\"question_id\":\"$NONANON_Q1\",\"option_ids\":[\"$NONANON_Q1_O1\"]},
    {\"question_id\":\"$ANON_Q1\",\"option_ids\":[\"$ANON_Q1_O1\"]},
    {\"question_id\":\"$ANON_Q2\",\"option_ids\":[\"$ANON_Q2_O1\"]},
    {\"question_id\":\"$ANON_Q3\",\"value\":3,\"value_set\":true},
    {\"question_id\":\"$ANON_Q4\",\"text\":\"ok\"}
  ]}"
expect_code "R22: foreign question_id" 400

# R23 — wrong payload: send option_ids for a scale question
do_req POST "/polls/$ANON_ID/vote" \
  "{\"answers\":[
    {\"question_id\":\"$ANON_Q1\",\"option_ids\":[\"$ANON_Q1_O1\"]},
    {\"question_id\":\"$ANON_Q2\",\"option_ids\":[\"$ANON_Q2_O1\"]},
    {\"question_id\":\"$ANON_Q3\",\"option_ids\":[\"$ANON_Q1_O1\"],\"value_set\":true},
    {\"question_id\":\"$ANON_Q4\",\"text\":\"ok\"}
  ]}"
expect_code "R23: wrong payload for scale" 400

# ---------- R24–R29: HasVoted ----------

section "R24–R29: HasVoted"

# R24 — bad UUID
do_req GET /polls/not-a-uuid/vote
expect_code "R24: HasVoted bad UUID" 400

# R25 — poll does not exist
do_req GET "/polls/$RANDOM_UUID/vote"
expect_code "R25: HasVoted poll not found" 404

# R26 — no JWT → false
do_req GET "/polls/$NONANON_ID/vote"
expect_code "R26: no JWT" 200
expect_jq    "R26: has_voted == false" '.has_voted == false'

# R27 — JWT, alice has not voted on nonanon → false
do_req GET "/polls/$NONANON_ID/vote" "" "$ALICE_TOKEN"
expect_code "R27: alice on nonanon, not voted" 200
expect_jq    "R27: has_voted == false" '.has_voted == false'

# R28 — JWT, owner has voted on nonanon (R5) → true
do_req GET "/polls/$NONANON_ID/vote" "" "$OWNER_TOKEN"
expect_code "R28: owner on nonanon, voted" 200
expect_jq    "R28: has_voted == true" '.has_voted == true'

# R29 — JWT present, but user's prior vote was anonymous.
# On $ANON_ID: there are anonymous votes (user_id = NULL, R6/R9a)
# and alice's identified vote (R7). Owner has NOT voted on $ANON_ID.
# Owner checks HasVoted — should be false, even though the poll has votes.
do_req GET "/polls/$ANON_ID/vote" "" "$OWNER_TOKEN"
expect_code "R29: owner on anon poll, only anonymous votes exist for others" 200
expect_jq    "R29: has_voted == false for owner" '.has_voted == false'

# ---------- cleanup ----------

section "Cleanup"
psql_t "DELETE FROM users WHERE id IN ('$OWNER_ID', '$ALICE_ID');" >/dev/null
echo "  removed test users (cascade cleaned polls, questions, options, votes)"

# ---------- summary ----------

echo
echo "${CYAN}### Summary${NC}"
echo "  ${GREEN}PASS${NC}: $PASS"
echo "  ${RED}FAIL${NC}: $FAIL"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
exit 0
