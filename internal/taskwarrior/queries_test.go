package taskwarrior

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortByPosition(t *testing.T) {
	tasks := []Task{
		{UUID: "cc", Description: "third", Position: "300"},
		{UUID: "aa", Description: "first", Position: "100"},
		{UUID: "bb", Description: "second", Position: "200"},
	}
	sortByPosition(tasks)

	assert.Equal(t, "first", tasks[0].Description)
	assert.Equal(t, "second", tasks[1].Description)
	assert.Equal(t, "third", tasks[2].Description)
}

func TestActiveTasksByOwner_PassesFilterArgs(t *testing.T) {
	var gotArgs []string
	orig := exportTasksByFilterFn
	exportTasksByFilterFn = func(args ...string) ([]Task, error) {
		gotArgs = args
		return []Task{{UUID: "t1"}}, nil
	}
	t.Cleanup(func() { exportTasksByFilterFn = orig })

	tasks, err := ActiveTasksByOwner("inke")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
	want := []string{"status:pending", "+ACTIVE", "owner:inke"}
	if !slicesEqual(gotArgs, want) {
		t.Errorf("filter args = %v, want %v", gotArgs, want)
	}
}

func TestCountActiveTasksByOwner_ReturnsLen(t *testing.T) {
	orig := exportTasksByFilterFn
	exportTasksByFilterFn = func(args ...string) ([]Task, error) {
		return []Task{{UUID: "t1"}, {UUID: "t2"}, {UUID: "t3"}}, nil
	}
	t.Cleanup(func() { exportTasksByFilterFn = orig })

	count, err := CountActiveTasksByOwner("inke")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestCountActiveTasksByOwner_PropagatesError(t *testing.T) {
	orig := exportTasksByFilterFn
	exportTasksByFilterFn = func(args ...string) ([]Task, error) {
		return nil, fmt.Errorf("boom")
	}
	t.Cleanup(func() { exportTasksByFilterFn = orig })

	_, err := CountActiveTasksByOwner("inke")
	if err == nil {
		t.Fatal("expected error to propagate")
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSortByPosition_Empty(t *testing.T) {
	var tasks []Task
	sortByPosition(tasks) // should not panic
	assert.Empty(t, tasks)
}

func TestSortByPosition_SingleTask(t *testing.T) {
	tasks := []Task{{UUID: "aa", Description: "only", Position: "100"}}
	sortByPosition(tasks)
	assert.Equal(t, 1, len(tasks))
}
