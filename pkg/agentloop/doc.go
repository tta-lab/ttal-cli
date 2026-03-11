// Package agentloop provides a reusable stateless agent loop.
//
// Run() executes one agent loop iteration: prompt → LLM → tool calls → response.
// The caller provides conversation history, a system prompt, tools, and an optional
// sandbox env. No persistence — the caller receives StepMessages and handles storage.
//
// Plane: shared
package agentloop
