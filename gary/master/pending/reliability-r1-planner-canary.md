# Reliability R1 planner canary

This is a planning-path canary, not an implementation job.

Inspect the supplied repository snapshot.

If both of these are visibly present in the repository state:
- README.md describes Reliability R1 or direct control-plane Yardmaster planning.
- docs/reliability-r1.md exists.

then return:
- done=true
- a short summary saying the Reliability R1 planner canary is satisfied
- jobs=[]

Do not schedule implementation workers.
Do not propose cleanup.
Do not change code.
