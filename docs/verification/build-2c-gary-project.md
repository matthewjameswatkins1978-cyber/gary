# Build 2C — Dedicated GARY Cloud Yard

**Date:** 2026-08-24 10:34:34 UTC  
**GARY Project ID:** gary-agent-yard  
**Project Number:** 418146339023  
**Display Name:** GARY Agent Yard  
**Region:** europe-west2  
**Billing Linked:** yes  
**Billing Account:** 011489-5956FD-8DE224  
**Parent Organization/Folder:** none (top-level project)

## Enabled APIs

- batch.googleapis.com
- compute.googleapis.com
- logging.googleapis.com
- billingbudgets.googleapis.com

## Budget

- **Name:** GARY Yard Gross Usage
- **Amount:** 75 GBP
- **Calendar Period:** MONTH
- **Credit Treatment:** EXCLUDE_ALL_CREDITS
- **Thresholds:** 50%, 75%, 90%, 100% (current spend)

## Heartbeat

- **Job ID:** gary-smoke-20260824-103434
- **Job UID:** gary-smoke-2026082-1fbcfb9a-7829-4b030
- **Final State:** SUCCEEDED
- **Heartbeat Verified:** yes
- **Heartbeat Marker:** GARY_HEARTBEAT_1787564074560458900
- **Duration:** ~79 seconds

## Parked State Verification

- **Compute Engine VMs running:** 0
- **APIs enabled:** yes (all 4 required APIs)
- **Billing linked:** yes
- **Project active:** yes
- **No active workers:** confirmed

## Summary

GARY now has a dedicated Google Cloud project (`gary-agent-yard`) with billing
linked, required APIs enabled, and a $75/month budget alerting guardrail. The
Batch heartbeat was verified against this project. No compute resources remain
running. The project is parked and ready for reuse.

## No Secrets

This document contains no credentials, keys, or tokens. All cloud access
uses the existing authenticated gcloud account.
