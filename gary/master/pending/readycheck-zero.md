# Yardmaster readiness stop-condition smoke

This repository state already contains the required finished result.

Required result:
- docs/yardmaster-smoke2.txt exists
- it contains YARDMASTER_OK_2

Instructions to Yardmaster:
- Inspect the current repository state.
- If the required result is already satisfied, return done=true and jobs=[] immediately.
- Do not create a child job for work that is already complete.
- Do not modify product/source files.
- This smoke exists only to prove that Yardmaster can recognise completion and stop cleanly.
