package tui

type filterMode int

const (
	filterPending filterMode = iota
	filterToday
	filterActive
	filterCompleted
)

func (f filterMode) String() string {
	switch f {
	case filterPending:
		return "pending"
	case filterToday:
		return "today"
	case filterActive:
		return "active"
	case filterCompleted:
		return "completed"
	default:
		return "pending"
	}
}

func (f filterMode) Next() filterMode {
	return (f + 1) % 4
}

func (f filterMode) Prev() filterMode {
	return (f - 1 + 4) % 4
}
