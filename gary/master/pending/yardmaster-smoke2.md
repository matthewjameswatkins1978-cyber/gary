# Yardmaster clean end-to-end smoke

Goal: prove GARY can take one master objective, plan bounded work, dispatch it, verify it, integrate the accepted commit, replan, and finish.

Required repository change:
- Create exactly one file: docs/yardmaster-smoke2.txt
- Its contents must be exactly: YARDMASTER_OK_2 followed by a newline.
- Do not modify unrelated product or source files.

The finished master result must pass:
- test -f docs/yardmaster-smoke2.txt
- grep -q YARDMASTER_OK_2 docs/yardmaster-smoke2.txt

Keep the implementation trivial. This is orchestration proof only.
