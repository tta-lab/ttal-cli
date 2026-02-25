package daemon

import (
	"testing"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/runtime"
)

func TestQuestionStoreAddGetRemove(t *testing.T) {
	store := newQuestionStore()

	batch := &QuestionBatch{
		AgentName: "test",
		Questions: []runtime.Question{{Text: "Q1"}},
		Answers:   make(map[int]string),
		CreatedAt: time.Now(),
	}

	shortID := store.add(batch)
	if shortID == "" {
		t.Fatal("expected non-empty shortID")
	}
	if batch.ShortID != shortID {
		t.Errorf("batch.ShortID = %q, want %q", batch.ShortID, shortID)
	}

	got, ok := store.get(shortID)
	if !ok || got != batch {
		t.Error("expected to find batch after add")
	}

	store.remove(shortID)
	_, ok = store.get(shortID)
	if ok {
		t.Error("expected batch removed")
	}
}

func TestQuestionStoreCleanup(t *testing.T) {
	store := newQuestionStore()

	old := &QuestionBatch{
		AgentName: "old",
		Questions: []runtime.Question{{Text: "Q"}},
		Answers:   make(map[int]string),
		CreatedAt: time.Now().Add(-1 * time.Hour),
	}
	fresh := &QuestionBatch{
		AgentName: "fresh",
		Questions: []runtime.Question{{Text: "Q"}},
		Answers:   make(map[int]string),
		CreatedAt: time.Now(),
	}

	oldID := store.add(old)
	freshID := store.add(fresh)

	store.cleanup(30 * time.Minute)

	if _, ok := store.get(oldID); ok {
		t.Error("expected old batch to be cleaned up")
	}
	if _, ok := store.get(freshID); !ok {
		t.Error("expected fresh batch to survive cleanup")
	}
}

func TestAllAnswered(t *testing.T) {
	batch := &QuestionBatch{
		Questions: []runtime.Question{
			{Text: "Q1"},
			{Text: "Q2"},
		},
		Answers: make(map[int]string),
	}

	if batch.AllAnswered() {
		t.Error("expected AllAnswered=false with no answers")
	}

	batch.Answers[0] = "A1"
	if batch.AllAnswered() {
		t.Error("expected AllAnswered=false with 1/2 answers")
	}

	batch.Answers[1] = "A2"
	if !batch.AllAnswered() {
		t.Error("expected AllAnswered=true with 2/2 answers")
	}
}

func TestAdvanceToNextUnanswered(t *testing.T) {
	t.Run("advances forward", func(t *testing.T) {
		batch := &QuestionBatch{
			Questions:   []runtime.Question{{Text: "Q1"}, {Text: "Q2"}, {Text: "Q3"}},
			Answers:     map[int]string{0: "A1"},
			CurrentPage: 0,
		}
		advanceToNextUnanswered(batch)
		if batch.CurrentPage != 1 {
			t.Errorf("CurrentPage = %d, want 1", batch.CurrentPage)
		}
	})

	t.Run("wraps around", func(t *testing.T) {
		batch := &QuestionBatch{
			Questions:   []runtime.Question{{Text: "Q1"}, {Text: "Q2"}, {Text: "Q3"}},
			Answers:     map[int]string{1: "A2", 2: "A3"},
			CurrentPage: 2,
		}
		advanceToNextUnanswered(batch)
		if batch.CurrentPage != 0 {
			t.Errorf("CurrentPage = %d, want 0", batch.CurrentPage)
		}
	})

	t.Run("all answered stays on last", func(t *testing.T) {
		batch := &QuestionBatch{
			Questions:   []runtime.Question{{Text: "Q1"}, {Text: "Q2"}},
			Answers:     map[int]string{0: "A1", 1: "A2"},
			CurrentPage: 0,
		}
		advanceToNextUnanswered(batch)
		if batch.CurrentPage != 1 {
			t.Errorf("CurrentPage = %d, want 1 (last question)", batch.CurrentPage)
		}
	})

	t.Run("skips answered forward", func(t *testing.T) {
		batch := &QuestionBatch{
			Questions:   []runtime.Question{{Text: "Q1"}, {Text: "Q2"}, {Text: "Q3"}, {Text: "Q4"}},
			Answers:     map[int]string{0: "A1", 1: "A2"},
			CurrentPage: 0,
		}
		advanceToNextUnanswered(batch)
		if batch.CurrentPage != 2 {
			t.Errorf("CurrentPage = %d, want 2", batch.CurrentPage)
		}
	})
}

func TestCustomAnswerStore(t *testing.T) {
	store := newCustomAnswerStore()

	// Set and getAndClear
	store.set(123, &customAnswerState{ShortID: "abc", QuestionIdx: 0, SetAt: time.Now()})

	state, ok := store.getAndClear(123)
	if !ok || state.ShortID != "abc" {
		t.Error("expected to get state after set")
	}

	// Should be cleared
	_, ok = store.getAndClear(123)
	if ok {
		t.Error("expected state cleared after getAndClear")
	}

	// Clear explicit
	store.set(456, &customAnswerState{ShortID: "def"})
	store.clear(456)
	_, ok = store.getAndClear(456)
	if ok {
		t.Error("expected state cleared after clear")
	}
}
