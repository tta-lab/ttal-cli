package ask

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tta-lab/logos"
)

func TestBuildAllowedPaths(t *testing.T) {
	tests := []struct {
		name  string
		mode  Mode
		param ModeParams
		want  []logos.AllowedPath
	}{
		{
			name:  "project mode returns project path",
			mode:  ModeProject,
			param: ModeParams{ProjectPath: "/path/to/project"},
			want:  []logos.AllowedPath{{Path: "/path/to/project", ReadOnly: true}},
		},
		{
			name:  "repo mode returns repo local path",
			mode:  ModeRepo,
			param: ModeParams{RepoLocalPath: "/path/to/repo"},
			want:  []logos.AllowedPath{{Path: "/path/to/repo", ReadOnly: true}},
		},
		{
			name:  "general mode with working dir returns it",
			mode:  ModeGeneral,
			param: ModeParams{WorkingDir: "/my/cwd"},
			want:  []logos.AllowedPath{{Path: "/my/cwd", ReadOnly: true}},
		},
		{
			name:  "general mode with empty working dir returns nil",
			mode:  ModeGeneral,
			param: ModeParams{WorkingDir: ""},
			want:  nil,
		},
		{
			// Security invariant: URL and web modes must never get filesystem access.
			name:  "url mode returns nil (no filesystem access)",
			mode:  ModeURL,
			param: ModeParams{WorkingDir: "/should/be/ignored"},
			want:  nil,
		},
		{
			// Security invariant: URL and web modes must never get filesystem access.
			name:  "web mode returns nil (no filesystem access)",
			mode:  ModeWeb,
			param: ModeParams{WorkingDir: "/should/be/ignored"},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAllowedPaths(tt.mode, tt.param)
			assert.Equal(t, tt.want, got)
		})
	}
}
