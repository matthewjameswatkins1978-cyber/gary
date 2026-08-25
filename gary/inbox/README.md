# GARY inbox

Put one bounded job manifest in `pending/`.  The GitHub workflow is triggered
only by changes there.  It records status in `status/<id>.json`; final states
are never dispatched again.

```json
{
  "id": "small-safe-job",
  "repo": "https://github.com/matthewjameswatkins1978-cyber/example.git",
  "base_branch": "master",
  "task": "Make one bounded change.",
  "acceptance": ["go test ./..."]
}
```

Run `gary inbox validate gary/inbox/pending` locally to validate manifests.
Only HTTPS repositories owned by `matthewjameswatkins1978-cyber` and direct,
shell-free acceptance commands are accepted. Job IDs use lowercase letters,
digits, and hyphens and are limited to 30 characters. See
`docs/github-oidc-setup.md` before enabling dispatch.
