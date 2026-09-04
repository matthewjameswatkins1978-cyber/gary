package batch

import "strings"

func singleAgentCodingWorkerScript(cfg CodingJobConfig) string {
	r := strings.NewReplacer(
		"__MODEL__", cfg.Model,
		"__REPO__", cfg.RepoURL,
		"__BASE__", cfg.BaseBranch,
		"__BRANCH__", cfg.GaryBranch,
		"__ACCEPTANCE__", cfg.Acceptance,
	)
	return r.Replace(singleAgentWorkerTemplate)
}

const singleAgentWorkerTemplate = `#!/bin/bash
set -e -o pipefail

export GEMINI_CLI_TRUST_WORKSPACE=true
export CLOUDSDK_PYTHON=/usr/local/bin/python
export GEMINI_CLI_SYSTEM_SETTINGS_PATH=/workspace/gemini-system-settings.json

# One assigned model, one model attempt. Stronger-model escalation belongs to
# the human rather than hidden provider routing.
cat > /workspace/gemini-system-settings.json <<'JSON'
{"general":{"retryFetchErrors":false,"maxAttempts":1,"plan":{"modelRouting":false}}}
JSON

echo "GARY SINGLE AGENT STARTING"
echo "Time: $(date -u +%Y-%m-%dT%H:%M:%SZ)"

if [ -n "$GARY_GITHUB_TOKEN" ]; then
  export GIT_CONFIG_COUNT=1
  export GIT_CONFIG_KEY_0=http.https://github.com/.extraheader
  export GIT_CONFIG_VALUE_0="Authorization: Basic $(printf 'x-access-token:%s' "$GARY_GITHUB_TOKEN" | base64 | tr -d '\n')"
fi

git clone __REPO__ /workspace/repo
cd /workspace/repo
git config --global user.email "gary@agent-yard.local"
git config --global user.name "GARY"
git checkout __BASE__
git checkout -b __BRANCH__

echo "$GARY_PROMPT_B64" | base64 -d > /workspace/gary_prompt.txt
: > /workspace/usage.jsonl
: > /workspace/acceptance_output.txt
: > /workspace/blocker.json

echo "Running autonomous Gemini agent..."
set +e
gemini -m __MODEL__ --yolo --output-format stream-json -p "$(cat /workspace/gary_prompt.txt)" 2>&1 | tee /workspace/usage.jsonl
GEMINI_EXIT=${PIPESTATUS[0]}
set -e
echo "Gemini CLI exit code: $GEMINI_EXIT"

# The model may request human action only through this structured, disposable
# handoff. It never becomes part of the candidate repository.
if [ -f .gary-blocked.json ]; then
  cp .gary-blocked.json /workspace/blocker.json
  rm -f .gary-blocked.json
fi

echo "Running acceptance checks..."
set +e
__ACCEPTANCE__ > /workspace/acceptance_output.txt 2>&1
ACCEPTANCE_EXIT=$?
set -e
cat /workspace/acceptance_output.txt || true
if [ $ACCEPTANCE_EXIT -eq 0 ]; then
  echo "ACCEPTANCE PASSED" | tee -a /workspace/acceptance_output.txt
else
  echo "ACCEPTANCE FAILED (exit code $ACCEPTANCE_EXIT)" | tee -a /workspace/acceptance_output.txt
fi
echo "GEMINI_EXIT=$GEMINI_EXIT" | tee -a /workspace/acceptance_output.txt

find . -type d \( -name .venv -o -name __pycache__ -o -name .pytest_cache -o -name .mypy_cache -o -name .ruff_cache -o -name .tox -o -name .nox -o -name .swiftpm -o -name .build -o -name target -o -name DerivedData \) -prune -exec rm -rf {} + 2>/dev/null || true
find . -type d -name '*.egg-info' -prune -exec rm -rf {} + 2>/dev/null || true

git add -A
git commit -m "GARY result" || true
git format-patch __BASE__ --stdout > /workspace/result.patch

# GCS is primary durable evidence. Marked log blocks are the independent
# transport fallback if GCS readback fails.
echo "GARY_PATCH_B64_START"
base64 /workspace/result.patch
echo "GARY_PATCH_B64_END"
echo "GARY_ACCEPTANCE_B64_START"
base64 /workspace/acceptance_output.txt
echo "GARY_ACCEPTANCE_B64_END"
echo "GARY_USAGE_B64_START"
base64 /workspace/usage.jsonl
echo "GARY_USAGE_B64_END"
echo "GARY_BLOCKER_B64_START"
base64 /workspace/blocker.json
echo "GARY_BLOCKER_B64_END"

gcloud storage cp /workspace/result.patch "$GARY_GCS_PATH/result.patch" 2>/dev/null || python3 -m gsutil cp /workspace/result.patch "$GARY_GCS_PATH/result.patch" 2>/dev/null || true
gcloud storage cp /workspace/acceptance_output.txt "$GARY_GCS_PATH/acceptance_output.txt" 2>/dev/null || python3 -m gsutil cp /workspace/acceptance_output.txt "$GARY_GCS_PATH/acceptance_output.txt" 2>/dev/null || true
gcloud storage cp /workspace/usage.jsonl "$GARY_GCS_PATH/usage.jsonl" 2>/dev/null || python3 -m gsutil cp /workspace/usage.jsonl "$GARY_GCS_PATH/usage.jsonl" 2>/dev/null || true
gcloud storage cp /workspace/blocker.json "$GARY_GCS_PATH/blocker.json" 2>/dev/null || python3 -m gsutil cp /workspace/blocker.json "$GARY_GCS_PATH/blocker.json" 2>/dev/null || true

echo "GARY SINGLE AGENT COMPLETE"
exit 0
`
