package taskwarrior

import (
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
