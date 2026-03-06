## PR Review Summary

## Critical Issues (2 found)

### 1. Bug - Wrong return value stops PR polling prematurely
**Confidence: 95/100**  
**File:** `internal/daemon/prwatch.go:247-250`

The function returns `(true, _)` to signal that polling should stop (per the contract on line 233). However, the log message says "waiting for review" implying continuation. When CI passes but no LGTM is present, you want to **continue polling** until an LGTM is added.

**Fix:** Change `return true, 0` to `return false, backoff(interval)` to continue polling.

### 2. Silent Failure - API errors indistinguishable from "no LGTM"
**Severity: CRITICAL**  
**File:** `internal/daemon/prwatch.go:398-402`

When `provider.ListComments()` fails (network error, auth failure, rate limiting), the function logs the error but returns `false`. This makes the error indistinguishable from "no LGTM comment exists."

**Impact:**
- The log message at line 247 misleads users: "waiting for review" when the real issue is an API failure
- PR will never be flagged as ready until a new CI run completes and the API call succeeds

**Fix:** Change signature to return `(bool, error)` and handle errors explicitly at call site.

---

## Suggestions (2 found)

### 1. Consider checking official PR reviews
**Confidence: 75/100**

The current implementation only checks `ListComments()`, which misses GitHub/Forgejo "Approved" reviews submitted through the official review UI. This is a limitation of the existing `gitprovider.Provider` interface.

### 2. Consider caching LGTM status
**Confidence: 60/100**

Every CI success check fetches all comments. For busy PRs with many comments, this could add latency.

---

## Strengths

- Clean, well-structured `hasLGTMComment` function
- Case-insensitive "LGTM" detection with `strings.ToLower()`
- Good log message providing clear context
- Follows existing code patterns and conventions

---

## Recommended Action

1. **Fix critical issue #1 first** — wrong return value stops polling
2. **Fix critical issue #2** — return error from `hasLGTMComment` to distinguish API failures
3. Re-run review after fixes