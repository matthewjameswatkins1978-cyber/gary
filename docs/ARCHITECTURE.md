# GARY Reborn architecture

Matthew's job is the durable unit. `gary do` preserves the exact packet and
creates one local job record. Codex App Server hosts the same Luna Yardmaster
thread across turns and restarts. GARY's Go yard owns only bounded worktrees,
worker processes, deterministic inspection/acceptance, event journaling and
human-boundary records.

The normal sequence is:

```text
front door -> preflight -> Luna turn -> worker(s) -> mechanical event
           -> same Luna thread -> independent acceptance -> serial integration
           -> final acceptance -> DONE
```

There is no AI polling loop. Dynamic tools remain the current App Server
boundary and `gary doctor` verifies the installed protocol schema, including
the experimental dynamic-tools capability on which this implementation relies.
Google Batch remains explicit legacy compatibility only.
