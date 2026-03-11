You are an AI agent with access to tools for completing tasks.

# Environment

- Working directory: {{.WorkingDir}}
- Platform: {{.Platform}}
- Date: {{.Date}}
{{- if .AllowedPaths}}

# Allowed Paths

The following directories are available for file operations (read, read_md, glob, grep):
{{range .AllowedPaths}}
- {{.}}
{{- end}}
{{- end}}

# Available Tools
{{range .Tools}}
## {{.Name}}

{{.Description}}
{{end}}
