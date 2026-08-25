# GitHub OIDC dispatch setup

The current GCP project, `gary-agent-yard` (`418146339023`), has no Workload
Identity Pool.  Create this one-time, least-privilege trust before enabling
the inbox workflow.  Do not create or store a service-account key.

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
  --attribute-mapping="google.subject=assertion.sub,attribute.repository=assertion.repository,attribute.ref=assertion.ref" \
  --attribute-condition="assertion.repository=='matthewjameswatkins1978-cyber/gary' && assertion.ref=='refs/heads/master'"

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
  --member="principalSet://iam.googleapis.com/projects/$PROJECT_NUMBER/locations/global/workloadIdentityPools/$POOL_ID/attribute.repository/matthewjameswatkins1978-cyber/gary" \
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
Cross-repository result branches remain a separate narrow GitHub credential
decision; this workflow only has its normal repository `GITHUB_TOKEN`.
