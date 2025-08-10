#!/usr/bin/env bash
set -euo pipefail

# Manager for the go40k duel game server
# Commands: start | stop | restart | status | logs | build | health

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
BIN_DIR="$ROOT_DIR/bin"
LOG_DIR="$ROOT_DIR/logs"
TMP_DIR="$ROOT_DIR/tmp"
BIN_GAME="$BIN_DIR/w40k-game"
PID_FILE="$TMP_DIR/game.pid"
PORT="${GAME_PORT:-8081}"
DATA_API_BASE="${DATA_API_BASE:-http://localhost:8080}"

mkdir -p "$BIN_DIR" "$LOG_DIR" "$TMP_DIR"

cmd_build() {
  echo "Building game binary -> $BIN_GAME"
  local ver time ld
  ver=$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo dev)
  time=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  ld="-s -w -X main.buildVersion=$ver -X main.buildTime=$time"
  (cd "$ROOT_DIR" && go build -ldflags "$ld" -o "$BIN_GAME" ./cmd/game)
}

is_running_pid() { local pid="${1:-}"; [[ -n "$pid" && -d "/proc/$pid" ]]; }

pid_from_port() {
  ss -ltnp 2>/dev/null | awk -v p=":$PORT" '$4 ~ p {print $NF}' | sed -n 's/.*pid=\([0-9]*\).*/\1/p' | head -n1 || true
}

cmd_status() {
  local pid=""
  if [[ -f "$PID_FILE" ]]; then pid=$(cat "$PID_FILE" 2>/dev/null || true); fi
  if is_running_pid "$pid"; then echo "Game is running (pid=$pid) on port $PORT"; return 0; fi
  pid=$(pid_from_port || true)
  if is_running_pid "$pid"; then echo "Game appears running (pid=$pid) on port $PORT)"; return 0; fi
  echo "Game is stopped"; return 1
}

cmd_start() {
  if cmd_status >/dev/null 2>&1; then echo "Already running; use restart"; return 0; fi
  if [[ ! -x "$BIN_GAME" ]]; then cmd_build; fi
  echo "Starting game on :$PORT (DATA_API_BASE=$DATA_API_BASE) ..."
  (cd "$ROOT_DIR"; nohup env GAME_PORT="$PORT" DATA_API_BASE="$DATA_API_BASE" "$BIN_GAME" >"$LOG_DIR/game.log" 2>&1 & echo $! >"$PID_FILE")
  sleep 0.5; cmd_status || { echo "Failed to start. See $LOG_DIR/game.log"; exit 1; }
}

cmd_stop() {
  local pid=""; if [[ -f "$PID_FILE" ]]; then pid=$(cat "$PID_FILE" 2>/dev/null || true); fi
  if is_running_pid "$pid"; then echo "Stopping game (pid=$pid) ..."; kill "$pid" 2>/dev/null || true; sleep 0.3; if is_running_pid "$pid"; then kill -9 "$pid" 2>/dev/null || true; fi; fi
  rm -f "$PID_FILE" || true
  local p; p=$(pid_from_port || true); if is_running_pid "$p"; then kill "$p" 2>/dev/null || true; sleep 0.2; is_running_pid "$p" && kill -9 "$p" 2>/dev/null || true; fi
  echo "Game stopped"
}

cmd_restart() { cmd_stop || true; cmd_start; }

cmd_logs() { : "${FOLLOW:=1}"; [[ -f "$LOG_DIR/game.log" ]] || { echo "No logs"; exit 0; }; if [[ "$FOLLOW" == "0" ]]; then cat "$LOG_DIR/game.log"; else tail -n 200 -f "$LOG_DIR/game.log"; fi }
cmd_health() { curl -fsS "http://localhost:$PORT/" >/dev/null && echo "OK" || { echo "FAIL"; exit 1; } }

usage(){ cat <<EOF
Usage: $(basename "$0") <command>
Commands: start | stop | restart | status | logs | build | health
Environment:
  GAME_PORT      Game port (default: 8081)
  DATA_API_BASE  Data API base URL (default: http://localhost:8080)
EOF
}

main(){ case "${1:-}" in
  start) shift; cmd_start "$@" ;;
  stop) shift; cmd_stop "$@" ;;
  restart) shift; cmd_restart "$@" ;;
  status) shift; cmd_status "$@" ;;
  logs) shift; cmd_logs "$@" ;;
  build) shift; cmd_build "$@" ;;
  health) shift; cmd_health "$@" ;;
  -h|--help|help|"") usage ;;
  *) echo "Unknown command: $1"; usage; exit 1 ;;
 esac }

main "$@"
