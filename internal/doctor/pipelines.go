package doctor

// defaultPipelinesContent is the starter pipelines.toml written by ttal doctor --fix.
// It defines standard/bugfix/hotfix pipelines for the common ttal workflow.
const defaultPipelinesContent = `# Pipeline definitions for ttal task go.
# Each pipeline defines a sequence of stages with role-based assignment and gates.
# Tasks are matched to pipelines by their tags.

[standard]
description = "Plan → Implement"
tags = ["feature", "refactor"]

[[standard.stages]]
name = "Plan"
assignee = "designer"
gate = "human"
reviewer = "plan-reviewer"
mode = "subagent"
comments = "task"

[[standard.stages]]
name = "Implement"
assignee = "worker"
gate = "auto"
reviewer = "pr-review-lead"
mode = "subagent"
comments = "pr"

[bugfix]
description = "Fix → Implement"
tags = ["bugfix"]

[[bugfix.stages]]
name = "Fix"
assignee = "fixer"
gate = "human"
reviewer = "plan-reviewer"
mode = "subagent"
comments = "task"

[[bugfix.stages]]
name = "Implement"
assignee = "worker"
gate = "auto"
reviewer = "pr-review-lead"
mode = "subagent"
comments = "pr"

[hotfix]
description = "Straight to implement"
tags = ["hotfix"]

[[hotfix.stages]]
name = "Implement"
assignee = "worker"
gate = "auto"
reviewer = "pr-review-lead"
mode = "subagent"
comments = "pr"
`
