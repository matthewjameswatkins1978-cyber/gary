# Yardmaster end-to-end smoke

Goal: prove the new GARY master-job controller can take one objective, plan bounded work, dispatch it, integrate an accepted worker commit, and stop.

Target change:
- Create exactly one file: docs/yardmaster-smoke.txt
- Its contents must be exactly: YARDMASTER_OK followed by a newline.
- Do not modify any other product or source file.

A correct finished repository passes:
- test -f docs/yardmaster-smoke.txt
- grep -q YARDMASTER_OK docs/yardmaster-smoke.txt

Keep this trivial. The purpose is orchestration proof, not clever coding.
