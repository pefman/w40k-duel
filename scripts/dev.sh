#!/usr/bin/env bash
set -euo pipefail

# Orchestrate API and Game servers together
# Commands: start | stop | restart | status | logs [api|game] | build | health

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
API_SH="$ROOT_DIR/scripts/api.sh"
GAME_SH="$ROOT_DIR/scripts/game.sh"

export API_PORT=${API_PORT:-8080}
export GAME_PORT=${GAME_PORT:-8081}
export DATA_API_BASE=${DATA_API_BASE:-http://localhost:$API_PORT}

require() { command -v "$1" >/dev/null 2>&1 || { echo "Missing dependency: $1"; exit 1; }; }
require curl

ensure_scripts() {
  [[ -x "$API_SH" ]] || { echo "api.sh missing or not executable: $API_SH"; exit 1; }
  [[ -x "$GAME_SH" ]] || { echo "game.sh missing or not executable: $GAME_SH"; exit 1; }
}

cmd_build(){ ensure_scripts; "$API_SH" build; "$GAME_SH" build; }
cmd_start(){ ensure_scripts; "$API_SH" start; "$GAME_SH" start; }
cmd_stop(){ ensure_scripts; "$API_SH" stop || true; "$GAME_SH" stop || true; }
cmd_restart(){ ensure_scripts; cmd_stop; cmd_start; }
cmd_status(){ ensure_scripts; echo "API:"; "$API_SH" status || true; echo "Game:"; "$GAME_SH" status || true; }
cmd_health(){ ensure_scripts; echo -n "API: "; "$API_SH" health || true; echo -n "Game: "; "$GAME_SH" health || true; }
cmd_logs(){ ensure_scripts; case "${1:-}" in api) shift; "$API_SH" logs "$@" ;; game) shift; "$GAME_SH" logs "$@" ;; *) echo "Specify logs api|game"; exit 1 ;; esac }

usage(){ cat <<EOF
Usage: $(basename "$0") <command>
Commands: start | stop | restart | status | build | health | logs [api|game]
Environment:
  API_PORT       API port (default: 8080)
  GAME_PORT      Game port (default: 8081)
  DATA_API_BASE  Data API base URL for game (default: http://localhost:$API_PORT)
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
