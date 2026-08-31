# GitHub OIDC dispatch setup

`gary-agent-yard` (`418146339023`) uses this one-time, least-privilege trust
for both the file inbox and the GitHub Issue control doorbell. Do not create
or store a service-account key. The provider is tied to immutable GitHub IDs:
the GARY repository (`1344689742`), the operator account (`249632099`), and
the `master` ref.

```bash
PROJECT_ID=gary-agent-yard
PROJECT_NUMBER=418146339023
POOL_ID=github
PROVIDER_ID=github
DISPATCHER=gary-github-dispatcher

gcloud iam service-accounts create "$DISPATCHER" --project="$PROJECT_ID" \
  --display-name="GARY GitHub inbox dispatcher"
gcloud iam workload-identity-pools create "$POOL_ID" --project="$PROJECT_ID" \
  --location=global --display-name="GitHub Actions"
gcloud iam workload-identity-pools providers create-oidc "$PROVIDER_ID" \
  --project="$PROJECT_ID" --location=global --workload-identity-pool="$POOL_ID" \
  --issuer-uri="https://token.actions.githubusercontent.com" \
  --attribute-mapping="google.subject=assertion.sub,attribute.repository_id=assertion.repository_id,attribute.actor_id=assertion.actor_id,attribute.ref=assertion.ref" \
  --attribute-condition="attribute.repository_id=='1344689742' && attribute.actor_id=='249632099' && attribute.ref=='refs/heads/master'"

gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:$DISPATCHER@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/batch.jobsEditor"
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:$DISPATCHER@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/logging.viewer"
gcloud storage buckets add-iam-policy-binding gs://gary-batch-results-418146339023 \
  --member="serviceAccount:$DISPATCHER@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/storage.objectViewer"
gcloud iam service-accounts add-iam-policy-binding \
  gary-batch-worker@$PROJECT_ID.iam.gserviceaccount.com \
  --member="serviceAccount:$DISPATCHER@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/iam.serviceAccountUser"
gcloud iam service-accounts add-iam-policy-binding \
  "$DISPATCHER@$PROJECT_ID.iam.gserviceaccount.com" \
  --member="principalSet://iam.googleapis.com/projects/$PROJECT_NUMBER/locations/global/workloadIdentityPools/$POOL_ID/attribute.repository_id/1344689742" \
  --role="roles/iam.workloadIdentityUser"
```

Then set these repository variables in `matthewjameswatkins1978-cyber/gary`:

```text
GARY_GCP_PROJECT=gary-agent-yard
GARY_GCP_REGION=europe-west2
GARY_WIF_PROVIDER=projects/418146339023/locations/global/workloadIdentityPools/github/providers/github
GARY_DISPATCH_SERVICE_ACCOUNT=gary-github-dispatcher@gary-agent-yard.iam.gserviceaccount.com
```

The dispatcher does not need Secret Manager access.  The existing Batch worker
continues to access its existing Gemini secret under its own service account.

## GitHub Issue control format

Open a new issue in this private repository with title exactly `GARY COMMAND`
and a body containing only this JSON object (a single `json` fenced block is
also accepted):

```json
{"version":1,"command":"doctor"}
```

The only v1 commands are `doctor`, `status`, `list`, `inspect`, `logs`,
`cancel`, `submit`, and `recover`. The workflow executes only newly opened
issues authored by `matthewjameswatkins1978-cyber`; the Go boundary repeats the
author, immutable repository-ID, title, JSON, enum, and protected-JC checks.
Results return as a comment on the same issue, which is then closed. Re-running
the workflow finds its marker and does not repeat a command.

`submit` and `recover` require this bounded job shape. `acceptance` is an enum,
not a shell field: `go_test_all`, `go_test_all_count_1`, `cargo_test`,
`cargo_test_all_features`, `npm_test`, or `python_pytest_q`.

```json
{
  "version": 1,
  "command": "submit",
  "job": {
    "id": "small-safe-job",
    "repo": "https://github.com/matthewjameswatkins1978-cyber/example.git",
    "base_branch": "master",
    "task": "Make one bounded change.",
    "acceptance": ["go_test_all"]
  }
}
```

`inspect`, `logs`, and `cancel` use `{"version":1,"command":"inspect","batch_job_id":"gary-run-example-1-20260825-120000"}`.
`logs` accepts `log_limit` from 1 to 50; `list` accepts `max_jobs` from 1 to
50. `recover` combines an exact `batch_job_id` with the same bounded `job`
shape and replays the base64 Cloud Logging patch through local acceptance.
JC-01 and JC-02 are protected from issue submission, cancellation, and
recovery.

## Dedicated GitHub App for accepted result branches

Read-only Issue commands require no GitHub App. Before `submit` or `recover`,
create a dedicated **GARY Result Pusher** GitHub App, install it only on GARY
and the selected target repositories, including `book-burner` for the current
private certification target, and grant only **Contents: Read and write** and
**Metadata: Read**. Generate one private key, then add its App ID as repository
variable `GARY_GITHUB_APP_ID` and its PEM as repository secret
`GARY_GITHUB_APP_PRIVATE_KEY`. The workflow exchanges it for a short-lived
installation token; no PAT is used and the token is never stored in Git.
