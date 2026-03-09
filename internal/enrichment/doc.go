// Package enrichment derives metadata from task descriptions.
//
// It generates kebab-case git branch names by extracting meaningful words
// from a task description, filtering stop words, and capping the result to
// a safe length. The caller is responsible for prepending any prefix (e.g.
// "worker/") to the returned slug.
//
// Plane: shared
package enrichment
