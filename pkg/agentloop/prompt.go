package agentloop

import (
	_ "embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed system.md.tpl
var systemPromptTemplate string

// systemPromptTmpl is parsed once at init — surfaces syntax errors at startup.
var systemPromptTmpl = template.Must(template.New("system").Parse(systemPromptTemplate))

// PromptData holds the runtime context used to render the default system prompt.
type PromptData struct {
	WorkingDir   string
	Platform     string
	Date         string
	AllowedPaths []string
	Tools        []ToolInfo
}

// ToolInfo holds a tool's name and full description for system prompt injection.
type ToolInfo struct {
	Name        string
	Description string
}

// BuildSystemPrompt renders the default system prompt with runtime context.
// The result is the base prompt — consumers append their own instructions after this.
func BuildSystemPrompt(data PromptData) (string, error) {
	var buf strings.Builder
	if err := systemPromptTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute system prompt template: %w", err)
	}
	return buf.String(), nil
}
