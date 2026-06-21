package model

// DreamStreamEventType describes one event in a model generation stream.
type DreamStreamEventType string

const (
	DreamStreamEventDelta     DreamStreamEventType = "delta"
	DreamStreamEventWarning   DreamStreamEventType = "warning"
	DreamStreamEventCompleted DreamStreamEventType = "completed"
	DreamStreamEventError     DreamStreamEventType = "error"
)

// DreamStreamEvent is the provider-independent streaming contract.
// Completed and Error are terminal events; every stream must emit exactly one.
type DreamStreamEvent struct {
	Type         DreamStreamEventType
	Content      string
	Message      string
	FinishReason string
	Provider     string
	Model        string
}

func (e DreamStreamEvent) Terminal() bool {
	return e.Type == DreamStreamEventCompleted || e.Type == DreamStreamEventError
}
