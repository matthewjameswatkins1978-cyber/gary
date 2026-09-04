# Workflows

The repository has one cheap CI workflow and one explicitly scoped live-fire
workflow. CI never invokes Gemini or Google Batch; the live-fire workflow is
armed only on `master` and runs the prepared packet once after merge.
