#!/usr/bin/env bash
# runner-toggle.sh — Enable/disable self-hosted GitHub Actions runners
# for TeachMeHIPAA/monorepo without re-registering.
#
# Usage:
#   runner-toggle.sh start   # Start all registered runners
#   runner-toggle.sh stop    # Stop all running runners gracefully
#   runner-toggle.sh status  # Show runner status (local + GitHub)
#   runner-toggle.sh restart # Stop then start all runners
#
# Environment:
#   RUNNER_COUNT  Number of runner instances (default: 4)
#   RUNNER_BASE   Base directory (default: ~/actions-runner)

set -euo pipefail

RUNNER_COUNT="${RUNNER_COUNT:-4}"
RUNNER_BASE="${RUNNER_BASE:-$HOME/actions-runner}"
REPO="TeachMeHIPAA/monorepo"

# Derive runner directory for instance N (1-based).
runner_dir() {
  local n="$1"
  if [[ "$n" -eq 1 ]]; then
    echo "$RUNNER_BASE"
  else
    echo "${RUNNER_BASE}-${n}"
  fi
}

# PID file for a runner instance.
pid_file() {
  echo "/tmp/github-runner-tmh-${1}.pid"
}

# Log file for a runner instance.
log_file() {
  echo "/tmp/runner-${1}.log"
}

# Check if a runner instance is running.
is_running() {
  local pf
  pf="$(pid_file "$1")"
  [[ -f "$pf" ]] && kill -0 "$(cat "$pf")" 2>/dev/null
}

cmd_start() {
  echo "Starting $RUNNER_COUNT self-hosted runners..."
  local started=0
  for i in $(seq 1 "$RUNNER_COUNT"); do
    local dir
    dir="$(runner_dir "$i")"
    if [[ ! -d "$dir" ]]; then
      echo "  [runner-$i] SKIP — directory $dir not found (not registered?)"
      continue
    fi
    if is_running "$i"; then
      echo "  [runner-$i] SKIP — already running (pid $(cat "$(pid_file "$i")"))"
      continue
    fi
    cd "$dir"
    nohup ./run.sh > "$(log_file "$i")" 2>&1 &
    local pid=$!
    echo "$pid" > "$(pid_file "$i")"
    echo "  [runner-$i] Started (pid $pid, log $(log_file "$i"))"
    started=$((started + 1))
  done
  echo "Done. $started runner(s) started."
}

cmd_stop() {
  echo "Stopping self-hosted runners..."
  local stopped=0
  for i in $(seq 1 "$RUNNER_COUNT"); do
    local pf
    pf="$(pid_file "$i")"
    if ! is_running "$i"; then
      echo "  [runner-$i] SKIP — not running"
      # Clean up stale PID file.
      rm -f "$pf"
      continue
    fi
    local pid
    pid="$(cat "$pf")"
    echo "  [runner-$i] Sending SIGINT to pid $pid..."
    kill -INT "$pid" 2>/dev/null || true
    # Wait up to 30s for graceful shutdown.
    local waited=0
    while kill -0 "$pid" 2>/dev/null && [[ $waited -lt 30 ]]; do
      sleep 1
      waited=$((waited + 1))
    done
    if kill -0 "$pid" 2>/dev/null; then
      echo "  [runner-$i] WARN — still running after 30s, sending SIGKILL"
      kill -9 "$pid" 2>/dev/null || true
    fi
    rm -f "$pf"
    echo "  [runner-$i] Stopped"
    stopped=$((stopped + 1))
  done
  echo "Done. $stopped runner(s) stopped."
}

cmd_status() {
  echo "=== Local runner processes ==="
  for i in $(seq 1 "$RUNNER_COUNT"); do
    local dir
    dir="$(runner_dir "$i")"
    if ! [[ -d "$dir" ]]; then
      echo "  [runner-$i] NOT INSTALLED ($dir missing)"
      continue
    fi
    if is_running "$i"; then
      echo "  [runner-$i] RUNNING (pid $(cat "$(pid_file "$i")"))"
    else
      echo "  [runner-$i] STOPPED"
    fi
  done

  echo ""
  echo "=== GitHub-registered runners ==="
  if command -v gh &>/dev/null; then
    gh api "repos/$REPO/actions/runners" \
      -q '.runners[] | "  \(.name)\t\(.status)\t\(.busy)"' 2>/dev/null \
      || echo "  (failed to query GitHub API)"
  else
    echo "  (gh CLI not available)"
  fi
}

case "${1:-}" in
  start)   cmd_start ;;
  stop)    cmd_stop ;;
  restart) cmd_stop; echo ""; cmd_start ;;
  status)  cmd_status ;;
  *)
    echo "Usage: $0 {start|stop|restart|status}"
    echo ""
    echo "Environment variables:"
    echo "  RUNNER_COUNT  Number of runner instances (default: 4)"
    echo "  RUNNER_BASE   Base directory (default: ~/actions-runner)"
    exit 1
    ;;
esac
