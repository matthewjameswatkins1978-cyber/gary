# Completion principle cloud canary

This is a bounded cloud canary for the GARY completion principle and the
Yardmaster-to-Worker handoff. Use the existing repository only as a disposable
canary target; do not change GARY or Suture.

The finished outcome is a minimal valid Rust crate in the repository, with a
Cargo manifest and one small library test. The Worker may choose the exact
file contents and implementation route. Before working, ensure the Rust
toolchain required by the checks is available, using the repository or runtime
toolchain when present and provisioning a compatible tool when needed.

The Yardmaster must assign one independently runnable Worker job. The Worker
must create the crate, run the stated checks, keep disposable build output out
of the commit, and return the actual check results and commit evidence.

Do not redesign the existing Go clamp implementation or modify its tests.
