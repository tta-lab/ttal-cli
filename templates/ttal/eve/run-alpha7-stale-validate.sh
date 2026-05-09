#!/usr/bin/env bash
set -euo pipefail

# α7 live smoke:
# Active paid DB state + stale client VALIDATE JWS must return current paid state
# and must not rewrite the entitlement row.
#
# Usage:
#   TOKEN=... ./run-alpha7-stale-validate.sh
#
# This script does not read ~/.config/flicknote/session.json, does not print TOKEN,
# and does not print the full Apple JWS. It fetches a stale signed_payload from
# Postgres at runtime and sends it directly to the validate endpoint.

USER_ID="${USER_ID:-fe4de0a3-0cf4-4d79-92e4-4be3fae2c634}"
BASE_URL="${BASE_URL:-https://dev-gw.flicknote.app}"
NAMESPACE="${NAMESPACE:-infra-dev}"
POSTGRES_POD="${POSTGRES_POD:-flicknote-1}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-postgres}"
DB_NAME="${DB_NAME:-supabase}"
DB_USER="${DB_USER:-postgres}"

if [[ -z "${TOKEN:-}" ]]; then
  echo "error: TOKEN env var is required" >&2
  echo "usage: TOKEN=... $0" >&2
  exit 2
fi
AUTH_TOKEN="${TOKEN#Bearer }"

response_file="$(mktemp)"
trap 'rm -f "$response_file"' EXIT

db_query() {
  local sql="$1"
  env KUBECONFIG= kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -c "$POSTGRES_CONTAINER" -- \
    psql -d "$DB_NAME" -U "$DB_USER" -P pager=off -At -F $'\t' -c "$sql"
}

echo "== α7 stale VALIDATE smoke =="
echo "user_id: $USER_ID"
echo "base_url: $BASE_URL"
echo

echo "== auth preflight =="
preflight_code="$(
  curl -sS \
    -o "$response_file" \
    -w '%{http_code}' \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    "$BASE_URL/api/v1/entitlement/$USER_ID"
)"
echo "http_status: $preflight_code"
if [[ "$preflight_code" != "200" ]]; then
  echo "error: token preflight failed; refresh TOKEN before running α7 smoke" >&2
  echo "response_body:" >&2
  cat "$response_file" >&2
  echo >&2
  exit 5
fi
echo "token accepted"
echo

echo "== before entitlement =="
before_entitlement="$(db_query "
SELECT tier, billing_cycle, expires_at, auto_renew, last_event_type, last_event_at, updated_at
FROM public.entitlements
WHERE id = '$USER_ID';
")"
echo "$before_entitlement"

is_active="$(db_query "
SELECT COALESCE((expires_at > now())::text, 'false')
FROM public.entitlements
WHERE id = '$USER_ID';
")"
if [[ "$is_active" != "true" ]]; then
  echo "error: entitlement is not active; α7 needs active paid DB state" >&2
  exit 3
fi

stale_meta="$(db_query "
SELECT transaction_id, product_id, expires_at, created_at
FROM public.subscription_events
WHERE user_id = '$USER_ID'
  AND event_type = 'VALIDATE'
  AND signed_payload IS NOT NULL
  AND expires_at < now()
ORDER BY created_at DESC
LIMIT 1;
")"

if [[ -z "$stale_meta" ]]; then
  echo "error: no stale VALIDATE signed_payload found for user $USER_ID" >&2
  exit 4
fi

IFS=$'\t' read -r stale_tx stale_product stale_expires stale_created <<< "$stale_meta"

echo
echo "== stale JWS selected =="
echo "transaction_id: $stale_tx"
echo "product_id: $stale_product"
echo "expires_at: $stale_expires"
echo "event_created_at: $stale_created"
echo "signed_payload: <hidden>"

before_count="$(db_query "
SELECT count(*)
FROM public.subscription_events
WHERE user_id = '$USER_ID'
  AND event_type = 'VALIDATE'
  AND transaction_id = '$stale_tx';
")"

stale_jws="$(db_query "
SELECT signed_payload
FROM public.subscription_events
WHERE user_id = '$USER_ID'
  AND event_type = 'VALIDATE'
  AND transaction_id = '$stale_tx'
  AND signed_payload IS NOT NULL
ORDER BY created_at DESC
LIMIT 1;
")"

echo
echo "== POST /api/v1/subscription/validate with stale JWS =="
http_code="$(
  jq -n --arg signedTransaction "$stale_jws" '{signedTransaction: $signedTransaction}' |
    curl -sS \
      -o "$response_file" \
      -w '%{http_code}' \
      -H "Authorization: Bearer $AUTH_TOKEN" \
      -H "Content-Type: application/json" \
      "$BASE_URL/api/v1/subscription/validate" \
      -d @-
)"

echo "http_status: $http_code"
echo "response_body:"
cat "$response_file"
echo

echo
echo "== after entitlement =="
after_entitlement="$(db_query "
SELECT tier, billing_cycle, expires_at, auto_renew, last_event_type, last_event_at, updated_at
FROM public.entitlements
WHERE id = '$USER_ID';
")"
echo "$after_entitlement"

after_count="$(db_query "
SELECT count(*)
FROM public.subscription_events
WHERE user_id = '$USER_ID'
  AND event_type = 'VALIDATE'
  AND transaction_id = '$stale_tx';
")"

echo
echo "== audit delta for stale transaction =="
echo "before_count: $before_count"
echo "after_count: $after_count"

echo
echo "== verdict hints =="
if [[ "$http_code" == "200" ]]; then
  echo "- HTTP 200: ok"
else
  echo "- HTTP status is not 200: investigate"
fi

if [[ "$before_entitlement" == "$after_entitlement" ]]; then
  echo "- entitlement row unchanged: ok"
else
  echo "- entitlement row changed: investigate"
fi

if [[ "$after_count" -gt "$before_count" ]]; then
  echo "- audit row appended for VALIDATE: ok"
else
  echo "- audit count did not grow: investigate validate audit insertion / notification_uuid NULL semantics"
fi

echo
echo "Optional log check:"
echo "  env KUBECONFIG= kubectl logs -n apps-dev deploy/subscription-service --since=5m | rg '$stale_tx|handleValidateSubscription|skip entitlement upsert'"
