package slogdriver

import "context"

// Label represents a key-value string pair.
type Label struct {
	Key   string
	Value string
}

// NewLabel returns a new Label from a key and a value.
func NewLabel(key, value string) Label {
	return Label{Key: key, Value: value}
}

// AddLabels returns a new Context with additional labels to be used in the log
// entries produced using that context.
//
// See https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#FIELDS.labels
func AddLabels(ctx context.Context, labels ...Label) context.Context {
	return context.WithValue(ctx, labelsContextKeyT{}, &labelContainer{
		Labels: labels,
		Parent: labelsFromContext(ctx),
	})
}

func labelsFromContext(ctx context.Context) *labelContainer {
	v, _ := ctx.Value(labelsContextKeyT{}).(*labelContainer)
	return v
}

type labelsContextKeyT struct{}

type labelContainer struct {
	Labels []Label
	Parent *labelContainer
}

func (l *labelContainer) Iterate(f func(Label)) {
	if l == nil {
		return
	}
	l.Parent.Iterate(f)
	for _, label := range l.Labels {
		f(label)
	}
}
