#!/usr/bin/env bash
set -euo pipefail

repo="${1:-ISCProduct/soc-ai-agent}"
target_branch="${2:-main}"
bypass_login="${3:-}"
ruleset_name="main-review-required-no-bypass"

existing_id="$(
  gh api "repos/${repo}/rulesets" --jq ".[] | select(.name == \"${ruleset_name}\") | .id" | head -n1 || true
)"

bypass_actors='[]'
if [[ -n "${bypass_login}" ]]; then
  bypass_id="$(gh api "users/${bypass_login}" --jq .id)"
  bypass_actors="[ { \"actor_id\": ${bypass_id}, \"actor_type\": \"User\", \"bypass_mode\": \"pull_request\" } ]"
fi

payload_file="$(mktemp)"
cat > "${payload_file}" <<EOF
{
  "name": "${ruleset_name}",
  "target": "branch",
  "enforcement": "active",
  "conditions": {
    "ref_name": {
      "include": ["refs/heads/${target_branch}"],
      "exclude": []
    }
  },
  "bypass_actors": ${bypass_actors},
  "rules": [
    { "type": "deletion" },
    { "type": "non_fast_forward" },
    {
      "type": "pull_request",
      "parameters": {
        "required_approving_review_count": 1,
        "dismiss_stale_reviews_on_push": true,
        "require_code_owner_review": false,
        "require_last_push_approval": false,
        "required_review_thread_resolution": false
      }
    }
  ]
}
EOF

if [[ -n "${existing_id}" ]]; then
  gh api --method PUT "repos/${repo}/rulesets/${existing_id}" --input "${payload_file}" >/dev/null
  echo "Updated ruleset: ${ruleset_name} (id: ${existing_id})"
else
  gh api --method POST "repos/${repo}/rulesets" --input "${payload_file}" >/dev/null
  echo "Created ruleset: ${ruleset_name}"
fi

rm -f "${payload_file}"
