package batch

import "strings"

// supervisedCodingWorkerScript keeps the Batch VM disposable while making the
// Gemini process inside it observable and steerable. The control plane writes
// control.json into the run's GCS prefix; this wrapper emits telemetry.json and
// reacts to CONTINUE, GUIDE, STOP_AND_REPLAN, and READY_FOR_INSPECTION.
func supervisedCodingWorkerScript(cfg CodingJobConfig) string {
	r := strings.NewReplacer(
		"__MODEL__", cfg.Model,
		"__REPO__", cfg.RepoURL,
		"__BASE__", cfg.BaseBranch,
		"__BRANCH__", cfg.GaryBranch,
		"__ACCEPTANCE__", cfg.Acceptance,
	)
	return r.Replace(supervisedWorkerTemplate)
}

const supervisedWorkerTemplate = `#!/bin/bash
set -e -o pipefail

export GEMINI_CLI_TRUST_WORKSPACE=true
export CLOUDSDK_PYTHON=/usr/local/bin/python
export GEMINI_CLI_SYSTEM_SETTINGS_PATH=/workspace/gemini-system-settings.json

cat > /workspace/gemini-system-settings.json <<'JSON'
{"general":{"retryFetchErrors":false,"maxAttempts":1,"plan":{"modelRouting":false}}}
JSON

echo "GARY SUPERVISED CODING WORKER STARTING"
echo "Time: $(date -u +%Y-%m-%dT%H:%M:%SZ)"

if [ -n "$GARY_GITHUB_TOKEN" ]; then
  export GIT_CONFIG_COUNT=1
  export GIT_CONFIG_KEY_0=http.https://github.com/.extraheader
  export GIT_CONFIG_VALUE_0="Authorization: Basic $(printf 'x-access-token:%s' "$GARY_GITHUB_TOKEN" | base64 | tr -d '\n')"
fi

echo "Cloning __REPO__..."
git clone __REPO__ /workspace/repo
cd /workspace/repo

echo "Rust: $(rustc --version)"
echo "Cargo: $(cargo --version)"
git config --global user.email "gary@agent-yard.local"
git config --global user.name "GARY Worker"

echo "Creating branch __BRANCH__ from __BASE__..."
git checkout __BASE__
git checkout -b __BRANCH__

if [ -n "$PREV_PATCH_GCS" ]; then
  echo "Applying previous patch from $PREV_PATCH_GCS..."
  (gcloud storage cp "$PREV_PATCH_GCS" /workspace/prev.patch 2>/dev/null || python3 -m gsutil cp "$PREV_PATCH_GCS" /workspace/prev.patch 2>/dev/null) &&
    git apply /workspace/prev.patch &&
    echo "Previous patch applied" ||
    echo "No previous patch to apply"
fi

echo "$GARY_PROMPT_B64" | base64 -d > /workspace/gary_prompt.txt
: > /workspace/usage.jsonl
: > /workspace/acceptance_output.txt
echo 0 > /workspace/last_control_generation

cat > /workspace/write_telemetry.py <<'PY'
import hashlib
import json
import os
import subprocess
import time

REPO = "/workspace/repo"

def run(args):
    p = subprocess.run(args, cwd=REPO, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    return p.stdout.strip()

def opt_int(name):
    raw = os.environ.get(name, "")
    if raw == "":
        return None
    try:
        return int(raw)
    except ValueError:
        return None

base = os.environ.get("GARY_BASE_BRANCH", "main")
turn_log = os.environ.get("GARY_TURN_LOG", "")
events = []
if turn_log and os.path.exists(turn_log):
    with open(turn_log, "r", encoding="utf-8", errors="replace") as fh:
        for line in fh:
            try:
                events.append(json.loads(line))
            except Exception:
                pass

session_id = os.environ.get("GARY_SESSION_ID", "")
tool_calls = {}
errors = []
for event in events:
    if event.get("type") == "init" and event.get("session_id"):
        session_id = event["session_id"]
    if event.get("type") == "tool_use":
        name = str(event.get("tool_name", "unknown"))
        tool_calls[name] = tool_calls.get(name, 0) + 1
    if event.get("type") == "error":
        errors.append(str(event.get("message", ""))[:300])

diff = subprocess.run(
    ["git", "diff", "--binary", base],
    cwd=REPO,
    stdout=subprocess.PIPE,
).stdout
status = run(["git", "status", "--short"])
stat = run(["git", "diff", "--stat", base])

accept_tail = ""
if os.path.exists("/workspace/acceptance_output.txt"):
    with open("/workspace/acceptance_output.txt", "r", encoding="utf-8", errors="replace") as fh:
        accept_tail = "".join(fh.readlines()[-30:])[-4000:]

started = int(os.environ.get("GARY_ACTIVE_STARTED", "0") or 0)
payload = {
    "schema_version": 1,
    "job_id": os.environ.get("GARY_JOB_ID", ""),
    "review_id": str(time.time_ns()),
    "phase": os.environ.get("GARY_TELEMETRY_PHASE", "running"),
    "turn": int(os.environ.get("GARY_TURN", "1")),
    "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    "elapsed_active_seconds": max(0, int(time.time()) - started),
    "session_id": session_id,
    "gemini_exit": opt_int("GARY_GEMINI_EXIT"),
    "acceptance_exit": opt_int("GARY_ACCEPTANCE_EXIT"),
    "changed_files": len([line for line in status.splitlines() if line.strip()]),
    "diff_bytes": len(diff),
    "diff_sha256": hashlib.sha256(diff).hexdigest(),
    "diff_stat": stat[-3000:],
    "git_status": status[-3000:],
    "acceptance_tail": accept_tail,
    "tool_calls": tool_calls,
    "errors": errors[-8:],
}
with open("/workspace/telemetry.json", "w", encoding="utf-8") as fh:
    json.dump(payload, fh, separators=(",", ":"))
PY

emit_telemetry() {
  export GARY_TELEMETRY_PHASE="$1"
  python3 /workspace/write_telemetry.py
  gcloud storage cp /workspace/telemetry.json "$GARY_GCS_PATH/telemetry.json" >/dev/null 2>&1 ||
    python3 -m gsutil cp /workspace/telemetry.json "$GARY_GCS_PATH/telemetry.json" >/dev/null 2>&1 ||
    true
}

read_control() {
  rm -f /workspace/control.new.json
  gcloud storage cp "$GARY_GCS_PATH/control.json" /workspace/control.new.json >/dev/null 2>&1 ||
    python3 -m gsutil cp "$GARY_GCS_PATH/control.json" /workspace/control.new.json >/dev/null 2>&1 ||
    return 1
}

control_generation() {
  if [ -f /workspace/control.new.json ]; then
    jq -r '.generation // 0' /workspace/control.new.json 2>/dev/null || echo 0
  else
    echo 0
  fi
}

apply_running_control() {
  local gemini_pid="$1"
  if ! read_control; then
    return 0
  fi
  local gen
  local last
  local action
  gen=$(control_generation)
  last=$(cat /workspace/last_control_generation 2>/dev/null || echo 0)
  if ! [ "$gen" -gt "$last" ] 2>/dev/null; then
    return 0
  fi
  echo "$gen" > /workspace/last_control_generation
  action=$(jq -r '.action // "CONTINUE"' /workspace/control.new.json)
  case "$action" in
    CONTINUE)
      echo "Yardmaster supervision: CONTINUE"
      ;;
    GUIDE)
      jq -r '.guidance // ""' /workspace/control.new.json > /workspace/pending_guidance.txt
      echo GUIDE > /workspace/control_action
      echo "Yardmaster supervision: GUIDE; interrupting current Gemini turn for directed resume"
      kill -TERM "$gemini_pid" 2>/dev/null || true
      ;;
    STOP_AND_REPLAN|READY_FOR_INSPECTION)
      echo "$action" > /workspace/control_action
      echo "Yardmaster supervision: $action"
      kill -TERM "$gemini_pid" 2>/dev/null || true
      ;;
  esac
}

ACTIVE_STARTED=$(date +%s)
export GARY_ACTIVE_STARTED="$ACTIVE_STARTED"
TURN=0
SESSION_ID=""
NEXT_PROMPT=/workspace/gary_prompt.txt
SUPERVISION_TERMINAL=""

while true; do
  TURN=$((TURN+1))
  export GARY_TURN="$TURN"
  TURN_LOG="/workspace/turn-$TURN.jsonl"
  export GARY_TURN_LOG="$TURN_LOG"
  : > "$TURN_LOG"
  rm -f /workspace/control_action /workspace/pending_guidance.txt

  echo "GARY SUPERVISED TURN $TURN START"
  if [ -z "$SESSION_ID" ]; then
    gemini -m __MODEL__ --yolo --output-format stream-json -p "$(cat "$NEXT_PROMPT")" > "$TURN_LOG" 2>&1 &
  else
    gemini -m __MODEL__ --resume "$SESSION_ID" --yolo --output-format stream-json -p "$(cat "$NEXT_PROMPT")" > "$TURN_LOG" 2>&1 &
  fi
  GEMINI_PID=$!

  (
    while kill -0 "$GEMINI_PID" 2>/dev/null; do
      export GARY_GEMINI_EXIT=""
      export GARY_ACCEPTANCE_EXIT=""
      emit_telemetry running
      sleep 60
    done
  ) &
  HEARTBEAT_PID=$!

  (
    while kill -0 "$GEMINI_PID" 2>/dev/null; do
      apply_running_control "$GEMINI_PID"
      sleep 15
    done
  ) &
  CONTROL_WATCH_PID=$!

  set +e
  wait "$GEMINI_PID"
  GEMINI_EXIT=$?
  set -e
  kill "$HEARTBEAT_PID" "$CONTROL_WATCH_PID" 2>/dev/null || true
  wait "$HEARTBEAT_PID" "$CONTROL_WATCH_PID" 2>/dev/null || true

  cat "$TURN_LOG" >> /workspace/usage.jsonl || true
  FOUND_SESSION=$(jq -r 'select(.type=="init") | .session_id // empty' "$TURN_LOG" 2>/dev/null | head -n1 || true)
  if [ -n "$FOUND_SESSION" ]; then
    SESSION_ID="$FOUND_SESSION"
  fi
  export GARY_SESSION_ID="$SESSION_ID"
  export GARY_GEMINI_EXIT="$GEMINI_EXIT"

  echo "Running acceptance checks after supervised turn $TURN..."
  : > /workspace/acceptance_output.txt
  set +e
  __ACCEPTANCE__ > /workspace/acceptance_output.txt 2>&1
  ACCEPTANCE_EXIT=$?
  set -e
  export GARY_ACCEPTANCE_EXIT="$ACCEPTANCE_EXIT"
  cat /workspace/acceptance_output.txt || true
  if [ "$ACCEPTANCE_EXIT" -eq 0 ]; then
    echo "ACCEPTANCE PASSED" | tee -a /workspace/acceptance_output.txt
  else
    echo "ACCEPTANCE FAILED (exit code $ACCEPTANCE_EXIT)" | tee -a /workspace/acceptance_output.txt
  fi

  if [ -f /workspace/control_action ]; then
    ACTION=$(cat /workspace/control_action)
    case "$ACTION" in
      GUIDE)
        if [ -s /workspace/pending_guidance.txt ]; then
          NEXT_PROMPT=/workspace/pending_guidance.txt
        else
          echo "Continue the assigned outcome from the current workspace. Preserve useful work and fix the observed problem." > /workspace/pending_guidance.txt
          NEXT_PROMPT=/workspace/pending_guidance.txt
        fi
        emit_telemetry turn_complete
        continue
        ;;
      STOP_AND_REPLAN|READY_FOR_INSPECTION)
        SUPERVISION_TERMINAL="$ACTION"
        emit_telemetry stopping
        break
        ;;
    esac
  fi

  if [ "$ACCEPTANCE_EXIT" -eq 0 ]; then
    PHASE=candidate_ready
  else
    PHASE=turn_complete
  fi
  emit_telemetry "$PHASE"

  BASELINE=$(cat /workspace/last_control_generation 2>/dev/null || echo 0)
  echo "Waiting for Yardmaster review after turn $TURN; control generation must exceed $BASELINE"
  while true; do
    if read_control; then
      GEN=$(control_generation)
      if [ "$GEN" -gt "$BASELINE" ] 2>/dev/null; then
        echo "$GEN" > /workspace/last_control_generation
        ACTION=$(jq -r '.action // "CONTINUE"' /workspace/control.new.json)
        case "$ACTION" in
          CONTINUE)
            echo "Continue working toward the assigned outcome from the current workspace. Preserve what works, address remaining failures, and finish with evidence." > /workspace/continue_prompt.txt
            NEXT_PROMPT=/workspace/continue_prompt.txt
            break
            ;;
          GUIDE)
            jq -r '.guidance // ""' /workspace/control.new.json > /workspace/pending_guidance.txt
            NEXT_PROMPT=/workspace/pending_guidance.txt
            break
            ;;
          STOP_AND_REPLAN|READY_FOR_INSPECTION)
            SUPERVISION_TERMINAL="$ACTION"
            break
            ;;
        esac
      fi
    fi
    sleep 10
  done
  if [ -n "$SUPERVISION_TERMINAL" ]; then
    break
  fi
done

if [ -z "$SUPERVISION_TERMINAL" ]; then
  SUPERVISION_TERMINAL=NONE
fi
echo "SUPERVISION_TERMINAL=$SUPERVISION_TERMINAL" | tee -a /workspace/acceptance_output.txt
echo "GEMINI_EXIT=$GEMINI_EXIT" | tee -a /workspace/acceptance_output.txt
echo "ACCEPTANCE_EXIT=$ACCEPTANCE_EXIT" | tee -a /workspace/acceptance_output.txt

find . -type d \( -name .venv -o -name __pycache__ -o -name .pytest_cache -o -name .mypy_cache -o -name .ruff_cache -o -name .tox -o -name .nox -o -name .swiftpm -o -name .build -o -name target -o -name DerivedData \) -prune -exec rm -rf {} + 2>/dev/null || true
find . -type d -name '*.egg-info' -prune -exec rm -rf {} + 2>/dev/null || true

echo "Committing supervised worker changes..."
git add -A
git commit -m "GARY worker result" || true

echo "Generating patch..."
git format-patch __BASE__ --stdout > /workspace/result.patch

echo "GARY_PATCH_B64_START"
base64 /workspace/result.patch
echo "GARY_PATCH_B64_END"
echo "GARY_ACCEPTANCE_B64_START"
base64 /workspace/acceptance_output.txt
echo "GARY_ACCEPTANCE_B64_END"
echo "GARY_USAGE_B64_START"
base64 /workspace/usage.jsonl
echo "GARY_USAGE_B64_END"

echo "Uploading supervised worker result..."
gcloud storage cp /workspace/result.patch "$GARY_GCS_PATH/result.patch" 2>/dev/null ||
  python3 -m gsutil cp /workspace/result.patch "$GARY_GCS_PATH/result.patch" ||
  true
gcloud storage cp /workspace/acceptance_output.txt "$GARY_GCS_PATH/acceptance_output.txt" 2>/dev/null ||
  python3 -m gsutil cp /workspace/acceptance_output.txt "$GARY_GCS_PATH/acceptance_output.txt" ||
  true
gcloud storage cp /workspace/usage.jsonl "$GARY_GCS_PATH/usage.jsonl" 2>/dev/null ||
  python3 -m gsutil cp /workspace/usage.jsonl "$GARY_GCS_PATH/usage.jsonl" ||
  true
emit_telemetry stopping

echo "GARY CODING WORKER COMPLETE"
echo "GARY SUPERVISED CODING WORKER COMPLETE"
exit 0
`
