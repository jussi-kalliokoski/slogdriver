package slogdriver

import "context"

// Trace contains tracing information used in logging.
type Trace struct {
	ID      string
	SpanID  string
	Sampled bool
}

func traceFromContext(ctx context.Context) Trace {
	v, _ := ctx.Value(traceContextKeyT{}).(Trace)
	return v
}

// Context returns a Context that stores the Trace.
func (trace Trace) Context(ctx context.Context) context.Context {
	return context.WithValue(ctx, traceContextKeyT{}, trace)
}

type traceContextKeyT struct{}
