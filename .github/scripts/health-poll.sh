#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <fqdn> [timeout_seconds] [interval_seconds]" >&2
  exit 2
fi

fqdn="$1"
timeout_seconds="${2:-180}"
interval_seconds="${3:-5}"
deadline=$((SECONDS + timeout_seconds))

while (( SECONDS < deadline )); do
  if curl -fsS "https://${fqdn}/healthz" >/dev/null && curl -fsS "https://${fqdn}/readyz" >/dev/null; then
    echo "health checks passed for ${fqdn}"
    exit 0
  fi

  sleep "$interval_seconds"
done

echo "health checks timed out for ${fqdn}" >&2
exit 1