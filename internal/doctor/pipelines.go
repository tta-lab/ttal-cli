package doctor

// defaultPipelinesContent is the starter pipelines.toml written by ttal doctor --fix.
// It defines standard/bugfix/hotfix pipelines for the common ttal workflow.
const defaultPipelinesContent = `# Pipeline definitions for ttal go.
# Each pipeline defines a sequence of stages with role-based assignment and gates.
# Tasks are matched to pipelines by their tags.
# Skills are loaded per-stage via ttal skill get <name>.

[standard]
description = "Plan → Implement"
tags = ["feature", "refactor"]

[[standard.stages]]
name = "Plan"
assignee = "designer"
gate = "human"
reviewer = "plan-review-lead"
skills = ["sp-planning", "flicknote"]

[[standard.stages]]
name = "Implement"
assignee = "coder"
gate = "auto"
reviewer = "pr-review-lead"

[bugfix]
description = "Fix → Implement"
tags = ["bugfix"]

[[bugfix.stages]]
name = "Fix"
assignee = "fixer"
gate = "human"
reviewer = "plan-review-lead"
skills = ["sp-debugging", "flicknote"]

[[bugfix.stages]]
name = "Implement"
assignee = "coder"
gate = "auto"
reviewer = "pr-review-lead"

[hotfix]
description = "Straight to implement"
tags = ["hotfix"]

[[hotfix.stages]]
name = "Implement"
assignee = "coder"
gate = "auto"
reviewer = "pr-review-lead"
`
