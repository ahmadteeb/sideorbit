#!/usr/bin/env bash
# Exits non-zero (and prints why) when Orbit isn't ready. Run it from another
# machine, for example every 5 minutes from cron:
#   */5 * * * * /path/uptime-check.sh https://orbit.example.com || mail -s "Orbit is down" you@example.com
set -euo pipefail
url="${1:?usage: uptime-check.sh https://orbit.example.com}"
body="$(curl -sS --max-time 10 -w '\n%{http_code}' "${url%/}/readyz")" || { echo "unreachable: $url"; exit 1; }
[ "${body##*$'\n'}" = 200 ] || { echo "not ready: ${body//$'\n'/ }"; exit 1; }
