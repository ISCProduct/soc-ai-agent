#!/bin/bash
# HSTS がフロントのレスポンスヘッダと CloudFront の両方にあることを検査する。
#
# どちらか一方だけだと、配信経路によって付かない（SOCAIAGENT-583）。
# 開発の localhost に付けるとブラウザが HTTP を拒否するので、Next は本番だけ。
#
# 使い方: ./automation/test/hsts-header-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NEXT="$ROOT/frontend/next.config.ts"
CF="$ROOT/infra/terraform/modules/cloudfront_app_proxy/main.tf"
fail=0

if ! grep -q "Strict-Transport-Security" "$NEXT"; then
  echo "FAIL next.config.ts に Strict-Transport-Security が無い" >&2
  fail=$((fail + 1))
fi
if ! grep -q "max-age=63072000; includeSubDomains" "$NEXT"; then
  echo "FAIL next.config.ts の HSTS が Backend と同じ max-age ではない" >&2
  fail=$((fail + 1))
fi
if ! grep -q '!isDev' "$NEXT"; then
  echo "FAIL next.config.ts の HSTS が開発モードでも付く形になっている" >&2
  fail=$((fail + 1))
else
  echo "ok   Next は本番だけ HSTS を付ける"
fi

if ! grep -q 'strict_transport_security' "$CF"; then
  echo "FAIL CloudFront に strict_transport_security が無い" >&2
  fail=$((fail + 1))
fi
if ! grep -q 'access_control_max_age_sec = 63072000' "$CF"; then
  echo "FAIL CloudFront の HSTS max-age が 63072000 ではない" >&2
  fail=$((fail + 1))
fi
attached=$(grep -c 'response_headers_policy_id' "$CF" || true)
if [ "$attached" -lt 2 ]; then
  echo "FAIL CloudFront のビヘイビアに response_headers_policy が付いていない（$attached 箇所）" >&2
  fail=$((fail + 1))
else
  echo "ok   CloudFront のビヘイビア $attached 箇所に HSTS ポリシーがある"
fi

if [ "$fail" -gt 0 ]; then
  echo "FAIL ($fail 件)" >&2
  exit 1
fi
echo "PASS (HSTS は Next 本番と CloudFront の両方にある)"
