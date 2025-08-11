#!/usr/bin/env bash
set -euo pipefail

# Manager for the API server only (game removed for clean slate)
# Commands: start | stop | restart | status | logs | build | health

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
API_SH="$ROOT_DIR/scripts/api.sh"

export API_PORT=${API_PORT:-8080}

require() { command -v "$1" >/dev/null 2>&1 || { echo "Missing dependency: $1"; exit 1; }; }
require curl

ensure_scripts() { [[ -x "$API_SH" ]] || { echo "api.sh missing or not executable: $API_SH"; exit 1; }; }

wait_for_api(){
  local url="http://localhost:$API_PORT/api/healthz" tries=40
  echo -n "Waiting for API health at $url"
  for i in $(seq 1 $tries); do
    if curl -fsS "$url" >/dev/null 2>&1; then echo " - OK"; return 0; fi
    echo -n "."; sleep 0.15
  done
  echo " - TIMEOUT"; return 1
}

cmd_build(){ ensure_scripts; "$API_SH" build; }
cmd_start(){ ensure_scripts; "$API_SH" start; wait_for_api || true; }
cmd_stop(){ ensure_scripts; "$API_SH" stop || true; }
cmd_restart(){ ensure_scripts; "$API_SH" restart; wait_for_api || true; }
cmd_status(){ ensure_scripts; echo "API:"; "$API_SH" status || true; }
cmd_health(){ ensure_scripts; echo -n "API: "; "$API_SH" health || true; }
cmd_logs(){ ensure_scripts; "$API_SH" logs "$@"; }

usage(){ cat <<EOF
Usage: $(basename "$0") <command>
Commands: start | stop | restart | status | build | health | logs
Environment:
  API_PORT       API port (default: 8080)
EOF
}

main(){ case "${1:-}" in
  start) shift; cmd_start "$@" ;;
  stop) shift; cmd_stop "$@" ;;
  restart) shift; cmd_restart "$@" ;;
  status) shift; cmd_status "$@" ;;
  build) shift; cmd_build "$@" ;;
  health) shift; cmd_health "$@" ;;
  logs) shift; cmd_logs "$@" ;;
  -h|--help|help|"") usage ;;
  *) echo "Unknown command: $1"; usage; exit 1 ;;
 esac }

main "$@"
