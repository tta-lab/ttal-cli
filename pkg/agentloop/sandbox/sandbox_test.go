package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSandboxBuildArgs(t *testing.T) {
	s := &Sandbox{BwrapPath: "bwrap"}

	args := s.buildArgs("echo hello", nil)

	// Verify core bwrap flags are present
	assert.Contains(t, args, "--ro-bind")
	assert.Contains(t, args, "--unshare-all")
	assert.Contains(t, args, "--share-net")
	assert.Contains(t, args, "--die-with-parent")

	// Verify command is last
	require.GreaterOrEqual(t, len(args), 3)
	assert.Equal(t, "bash", args[len(args)-3])
	assert.Equal(t, "-c", args[len(args)-2])
	assert.Equal(t, "echo hello", args[len(args)-1])
}

func TestSandboxBuildArgs_WithMounts(t *testing.T) {
	s := &Sandbox{BwrapPath: "bwrap"}
	cfg := &ExecConfig{
		MountDirs: []Mount{
			{Source: "/data", Target: "/data", ReadOnly: true},
			{Source: "/writable", Target: "/writable", ReadOnly: false},
		},
	}

	args := s.buildArgs("ls", cfg)

	// Verify read-only mount uses --ro-bind
	foundROBind := false
	for i, a := range args {
		if a == "--ro-bind" && i+2 < len(args) && args[i+1] == "/data" && args[i+2] == "/data" {
			foundROBind = true
		}
	}
	assert.True(t, foundROBind, "expected --ro-bind for /data")

	// Verify writable mount uses --bind
	foundBind := false
	for i, a := range args {
		if a == "--bind" && i+2 < len(args) && args[i+1] == "/writable" && args[i+2] == "/writable" {
			foundBind = true
		}
	}
	assert.True(t, foundBind, "expected --bind for /writable")
}

func TestBuildEnv(t *testing.T) {
	cfg := &ExecConfig{
		Env: []string{"FOO=bar", "BAZ=qux"},
	}

	env := buildEnv(cfg)

	assert.Contains(t, env, "PATH=/usr/bin:/usr/local/bin:/bin")
	assert.Contains(t, env, "HOME=/home/agent")
	assert.Contains(t, env, "FOO=bar")
	assert.Contains(t, env, "BAZ=qux")
}

func TestBuildEnv_Nil(t *testing.T) {
	env := buildEnv(nil)

	assert.Contains(t, env, "PATH=/usr/bin:/usr/local/bin:/bin")
	assert.Len(t, env, 3) // PATH, HOME, TERM
}

func TestTruncate(t *testing.T) {
	s := "hello"
	assert.Equal(t, "hello", truncate(s, 10))

	long := "12345678901234567890"
	result := truncate(long, 10)
	assert.Equal(t, "1234567890\n[output truncated]", result)
}

func TestExecDirect_AllowUnsandboxed(t *testing.T) {
	s := &Sandbox{
		BwrapPath:        "bwrap-nonexistent",
		AllowUnsandboxed: true,
	}

	ctx := t.Context()
	stdout, stderr, code, err := s.Exec(ctx, "echo hello", nil)

	require.NoError(t, err)
	assert.Equal(t, "hello\n", stdout)
	assert.Empty(t, stderr)
	assert.Equal(t, 0, code)
}

func TestExecDirect_Unsandboxed_Denied(t *testing.T) {
	s := &Sandbox{
		BwrapPath:        "bwrap-nonexistent",
		AllowUnsandboxed: false,
	}

	ctx := t.Context()
	_, _, _, err := s.Exec(ctx, "echo hello", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bwrap not found")
}

func TestContextWithExecConfig(t *testing.T) {
	cfg := &ExecConfig{Env: []string{"X=1"}}
	ctx := t.Context()

	ctx = ContextWithExecConfig(ctx, cfg)
	got := ExecConfigFromContext(ctx)

	require.NotNil(t, got)
	assert.Equal(t, cfg.Env, got.Env)
}

func TestExecConfigFromContext_Missing(t *testing.T) {
	ctx := t.Context()
	got := ExecConfigFromContext(ctx)
	assert.Nil(t, got)
}
