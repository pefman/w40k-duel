#!/usr/bin/env bash
set -euo pipefail

# Simple manager for the W40K API server.
# Commands: start | stop | restart | status | logs | build | health

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
BIN_DIR="$ROOT_DIR/bin"
LOG_DIR="$ROOT_DIR/logs"
TMP_DIR="$ROOT_DIR/tmp"
BIN="$BIN_DIR/w40k-api"
PID_FILE="$TMP_DIR/api.pid"
PORT="${API_PORT:-8080}"

mkdir -p "$BIN_DIR" "$LOG_DIR" "$TMP_DIR"

cmd_build() {
  echo "Building API binary -> $BIN"
  (cd "$ROOT_DIR" && go build -o "$BIN" ./cmd/api)
}

is_running_pid() {
  local pid="$1"
  [[ -n "$pid" ]] || return 1
  if kill -0 "$pid" 2>/dev/null; then
    return 0
  fi
  return 1
}

pid_from_port() {
  # Try to find a process listening on $PORT (cross-platform)
  # macOS: lsof; Linux: ss/lsof; generic: netstat
  local pid=""
  if command -v lsof >/dev/null 2>&1; then
    pid=$(lsof -nP -iTCP:"$PORT" -sTCP:LISTEN 2>/dev/null | awk 'NR>1 {print $2; exit}')
  fi
  if [[ -z "$pid" ]] && command -v ss >/dev/null 2>&1; then
    pid=$(ss -ltnp 2>/dev/null | awk -v p=":$PORT" '$4 ~ p {print $NF}' | sed -n 's/.*pid=\([0-9]*\).*/\1/p' | head -n1)
  fi
  if [[ -z "$pid" ]] && command -v netstat >/dev/null 2>&1; then
    pid=$(netstat -anv -p tcp 2>/dev/null | awk -v p=".$PORT" '$4 ~ p && $6 ~ /LISTEN/ {print $0; exit}' | sed -n 's/.*\.\([0-9][0-9]*\)[^0-9].*/\1/p')
  fi
  echo "$pid"
}

cmd_status() {
  local status="stopped"
  local pid=""
  if [[ -f "$PID_FILE" ]]; then
    pid=$(cat "$PID_FILE" 2>/dev/null || true)
    if is_running_pid "$pid"; then
      status="running"
      echo "API is $status (pid=$pid) on port $PORT"
      return 0
    fi
  fi
  # Fallback: check port
  pid=$(pid_from_port || true)
  if is_running_pid "$pid"; then
    echo "API appears running (pid=$pid) on port $PORT (no/old pidfile)"
  else
    echo "API is $status"
    return 1
  fi
}

cmd_start() {
  if cmd_status >/dev/null 2>&1; then
    echo "Already running; use restart to reload."
    return 0
  fi
  if [[ ! -x "$BIN" ]]; then
    cmd_build
  fi
  echo "Starting API on :$PORT ..."
  (cd "$ROOT_DIR"; nohup env API_PORT="$PORT" "$BIN" >"$LOG_DIR/api.log" 2>&1 & echo $! >"$PID_FILE")
  sleep 0.5
  cmd_status || { echo "Failed to start. See $LOG_DIR/api.log"; exit 1; }
}

cmd_stop() {
  local rc=0
  if [[ -f "$PID_FILE" ]]; then
    local pid
    pid=$(cat "$PID_FILE" 2>/dev/null || true)
    if is_running_pid "$pid"; then
      echo "Stopping API (pid=$pid) ..."
      kill "$pid" 2>/dev/null || true
      # Wait briefly, then force if needed
      for _ in {1..20}; do
        if ! is_running_pid "$pid"; then break; fi
        sleep 0.1
      done
      if is_running_pid "$pid"; then
        echo "Forcing stop (pid=$pid) ..."
        kill -9 "$pid" 2>/dev/null || true
      fi
    fi
    rm -f "$PID_FILE"
  fi
  # Ensure port is free if a stray process remains
  local p
  p=$(pid_from_port || true)
  if is_running_pid "$p"; then
    echo "Killing stray process on :$PORT (pid=$p) ..."
    kill "$p" 2>/dev/null || true
    sleep 0.2
    if is_running_pid "$p"; then
      kill -9 "$p" 2>/dev/null || true
    fi
  fi
  echo "API stopped"
  return $rc
}

cmd_restart() {
  cmd_stop || true
  cmd_build
  cmd_start
}

cmd_logs() {
  : "${FOLLOW:=1}"
  if [[ ! -f "$LOG_DIR/api.log" ]]; then
    echo "No log file yet at $LOG_DIR/api.log"
    exit 0
  fi
  if [[ "$FOLLOW" == "0" ]]; then
    cat "$LOG_DIR/api.log"
  else
    tail -n 200 -f "$LOG_DIR/api.log"
  fi
}

cmd_health() {
  curl -fsS "http://localhost:$PORT/api/healthz" || { echo; echo "health check failed"; exit 1; }
  echo
}

usage() {
  cat <<EOF
Usage: $(basename "$0") <command>

Commands:
  start      Build (if needed) and start the API on :${PORT}
  stop       Stop the API process
  restart    Restart the API
  status     Show API status
  logs       Tail the API logs (set FOLLOW=0 to print once)
  build      Build the API binary only
  health     Call GET /api/healthz on localhost:${PORT}

Environment:
  API_PORT   Port to listen on (default: 8080)
EOF
}

main() {
  local cmd="${1:-}"
  case "$cmd" in
    start)   shift; cmd_start "$@" ;;
    stop)    shift; cmd_stop "$@" ;;
    restart) shift; cmd_restart "$@" ;;
    status)  shift; cmd_status "$@" ;;
    logs)    shift; cmd_logs "$@" ;;
    build)   shift; cmd_build "$@" ;;
    health)  shift; cmd_health "$@" ;;
    -h|--help|help|"") usage ;;
    *) echo "Unknown command: $cmd"; usage; exit 1 ;;
  esac
}

main "$@"
