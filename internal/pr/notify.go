// Package pr handles GitHub/Forgejo PR operations.
//
// Plane: shared
package pr

import "fmt"

// BuildOwnerReviewMessage renders the notification sent to the task owner when
// a worker creates a PR. The owner reviews the PR against their plan using the
// sp-review-against-plan skill, then either advances the pipeline (LGTM → ttal go)
// or sends blockers back to the worker (NEED_WORK → ttal send via heredoc).
func BuildOwnerReviewMessage(prNum int64, prURL, title, worktree, hex, assignee string) string {
	return fmt.Sprintf(`📋 PR #%d ready for owner review — task %s

Title:    %s
URL:      %s
Worktree: %s

Review the PR against your plan:
  skill get sp-review-against-plan
  # in-scope undone = BLOCKING; cosmetic no-value = don't mention;
  # commit count/style is cosmetic (we squash-merge)

Verdict:
  LGTM      → ttal go %s
             # advances pipeline; pr-review-lead takes over

  NEED_WORK → send blockers to worker (heredoc — typically long + code):
             cat <<'EOF' | ttal send --to %s:%s
             <findings / blockers / code snippets>
             EOF
             # worker fixes, pushes; you re-review`, prNum, hex, title, prURL, worktree, hex, hex, assignee)
}
