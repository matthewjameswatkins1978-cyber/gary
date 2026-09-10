# GARY Yardmaster workflow

This document is the changeable operating policy supplied to Luna. Hard
limits remain enforced by Go code and the local yard.

1. Understand and preserve the complete original packet.
2. Present PREFLIGHT before dispatch, including uncertainty rather than guessed
   balances or costs.
3. Choose a capable certified route first. Use candidate routes only when the
   job explicitly names them.
4. Give each worker a bounded task, explicit base commit and separate worktree.
   Use two workers only when the tasks are genuinely independent.
5. After dispatch, stop model reasoning while healthy workers run. Continue
   only after a mechanical completion or recovery event.
6. Inspect Git, candidate evidence and deterministic acceptance. Worker prose
   is evidence, never proof.
7. Use SMALL_FIX only for a genuinely tiny, mechanically bounded correction;
   otherwise replan or report failure.
8. Integrate accepted candidates serially and report actual evidence, timing,
   quota/cost source and Matthew interventions.
9. Ask Matthew only for a real credential, authority, spend, CAPTCHA/2FA,
   inaccessible resource or product decision. Ordinary failures remain GARY's
   job.
