package enrichment

import (
	"strings"
	"unicode"
)

// maxBranchWords is the maximum number of words to include in a branch name.
const maxBranchWords = 4

// maxBranchLen is the maximum character length for a branch name.
const maxBranchLen = 50

var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "in": true, "on": true, "at": true,
	"to": true, "for": true, "of": true, "with": true, "from": true, "by": true,
	"as": true, "is": true, "are": true, "was": true, "were": true, "be": true,
	"been": true, "being": true, "that": true, "this": true, "it": true, "its": true,
	"and": true, "or": true, "but": true, "not": true, "so": true, "if": true,
	"then": true, "than": true, "when": true, "while": true, "should": true,
	"could": true, "would": true, "will": true, "can": true, "do": true,
	"does": true, "did": true, "has": true, "have": true, "had": true,
	"into": true, "also": true, "just": true, "about": true, "over": true,
	"after": true, "before": true, "between": true, "through": true, "during": true,
	"without": true, "within": true, "along": true, "across": true, "against": true,
	"upon": true, "onto": true, "toward": true, "towards": true, "until": true,
	"unless": true, "since": true, "because": true, "although": true, "though": true,
	"whether": true, "either": true, "neither": true, "both": true, "each": true,
	"every": true, "all": true, "any": true, "some": true, "no": true,
	"only": true, "very": true, "too": true, "quite": true, "rather": true,
	"already": true, "still": true, "yet": true,
}

// GenerateBranch derives a kebab-case branch name from a task description.
// Returns a short, descriptive branch name (e.g. "fix-auth-timeout-login").
// Returns an empty string if description is empty or contains no usable words.
// Does not include the "worker/" prefix — caller adds that.
func GenerateBranch(description string) string {
	words := extractWords(strings.ToLower(description))

	var meaningful []string
	for _, w := range words {
		if !stopWords[w] && len(w) > 1 {
			meaningful = append(meaningful, w)
		}
	}

	if len(meaningful) > maxBranchWords {
		meaningful = meaningful[:maxBranchWords]
	}

	// Fallback: if all words were stop words, use first 3 raw words
	if len(meaningful) == 0 && len(words) > 0 {
		meaningful = words
		if len(meaningful) > 3 {
			meaningful = meaningful[:3]
		}
	}

	branch := strings.Join(meaningful, "-")

	if len(branch) > maxBranchLen {
		cut := branch[:maxBranchLen]
		if idx := strings.LastIndex(cut, "-"); idx > 0 {
			cut = cut[:idx]
		}
		branch = cut
	}

	return branch
}

func extractWords(s string) []string {
	var words []string
	var current strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}
	return words
}
