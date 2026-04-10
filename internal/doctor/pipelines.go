package doctor

// defaultPipelinesContent is the starter pipelines.toml written by ttal doctor --fix.
// It defines standard/bugfix/hotfix pipelines for the common ttal workflow.
const defaultPipelinesContent = `# Pipeline definitions for ttal go.
# Each pipeline defines a sequence of stages with role-based assignment and gates.
# Tasks are matched to pipelines by their tags.
# Skills are declared per-role in roles.toml; role prompts carry
# "ttal skill get <name>" instructions so agents fetch methodology
# on demand (not auto-inlined at SessionStart -- exceeds CC hook size budget).

[standard]
description = "Plan → Implement"
tags = ["feature", "refactor"]

[[standard.stages]]
name = "Plan"
assignee = "designer"
gate = "human"
reviewer = "plan-review-lead"

[[standard.stages]]
name = "Implement"
assignee = "coder"
worker = true
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

[[bugfix.stages]]
name = "Implement"
assignee = "coder"
worker = true
gate = "auto"
reviewer = "pr-review-lead"

[hotfix]
description = "Straight to implement"
tags = ["hotfix"]

[[hotfix.stages]]
name = "Implement"
assignee = "coder"
worker = true
gate = "auto"
reviewer = "pr-review-lead"
`
