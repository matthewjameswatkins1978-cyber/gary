# Build 2D — First Google AI Worker

**Date:** 2026-08-24 15:05:47 UTC

## Cloud Setup

- **Project:** gary-agent-yard
- **Service Account:** gary-gemini-worker@gary-agent-yard.iam.gserviceaccount.com
- **API Key Type:** authorization key
- **Key Restriction:** generativelanguage.googleapis.com only
- **APIs Enabled:** generativelanguage, apikeys, iam

## Managed Agent

- **Agent ID:** antigravity-preview-05-2026
- **Underlying Model:** gemini-3.5-flash-lite
- **Thinking Level:** low
- **Max Token Budget:** 250,000

## Interaction

- **Interaction ID:** v1_Chd3RS1NYXJ1V005RzBrZFVQbWVDVXFRdxIXd0UtTWFydVdNOUcwa2RVUG1lQ1VxUXc
- **Environment ID:** 05905071a947fcde059b2027a44e645a
- **Status:** completed
- **Duration:** 208 seconds

## Token Usage

- **Input Tokens:** 149,350
- **Output Tokens:** 3,017
- **Thought Tokens:** 3,716
- **Cached Tokens:** 94,208
- **Tool-Use Tokens:** 0
- **Total Tokens:** 156,083
- **Service Tier:** standard

## Snapshot

- **Environment Reported Size:** 252,224,168 bytes (~252 MB)
- **Snapshot Format:** TAR (plain, not gzip)
- **Extraction:** streaming TAR reader, safe path enforcement

## Acceptance

- **Agent Completed:** yes
- **Snapshot Downloaded:** yes
- **Safe TAR Extraction:** yes (rejects absolute paths, .., symlinks escaping root)
- **Tests Unchanged:** yes
- **Local Tests Passed:** yes
- **Marker Verified:** yes (GARY_AGENT_SMOKE_OK)

## Go Availability

- Go was NOT pre-installed in the remote environment
- Agent downloaded and installed Go 1.24.0 automatically
- This contributed to ~252 MB snapshot size

## Verification

- `gofmt` — pass
- `go vet ./...` — pass
- `staticcheck ./...` — pass
- `go test ./...` — all packages pass

## Security

- **Exposed Key Revoked:** yes
- **New Key Created:** yes (same restrictions)
- **Key in Output:** no
- **Key in Results:** no
- **Key in Git:** no

## Parked State

- **Compute Engine VMs:** 0
- **Batch Jobs:** 0 active (1 old succeeded)
- **Gemini API Key:** retained (required for future use)
- **APIs Enabled:** retained

## No Secrets

This document contains no credentials, keys, or tokens.
