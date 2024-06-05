package slogdriver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime"

	"github.com/jussi-kalliokoski/goldjson"
)

// Config defines the Stackdriver configuration.
type Config struct {
	ProjectID string
	Level     slog.Leveler
}

// Handler is a handler that writes the log entries in the stackdriver logging
// JSON format.
type Handler struct {
	encoder      *goldjson.Encoder
	config       Config
	attrBuilders []func(ctx context.Context, h *Handler, l *goldjson.LineWriter, next func(context.Context) error) error
}

// NewHandler returns a new Handler.
func NewHandler(w io.Writer, config Config) *Handler {
	encoder := goldjson.NewEncoder(w)
	encoder.PrepareKey(fieldMessage)
	encoder.PrepareKey(fieldTimestamp)
	encoder.PrepareKey(fieldSeverity)
	encoder.PrepareKey(fieldSourceLocation)
	encoder.PrepareKey(fieldSourceFile)
	encoder.PrepareKey(fieldSourceLine)
	encoder.PrepareKey(fieldSourceFunction)
	encoder.PrepareKey(fieldTraceID)
	encoder.PrepareKey(fieldTraceSpanID)
	encoder.PrepareKey(fieldTraceSampled)
	encoder.PrepareKey(fieldLabels)
	return &Handler{
		encoder: encoder,
		config:  config,
	}
}

// Handle implements slog.Handler.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	l := h.encoder.NewLine()

	h.addMessage(ctx, l, &r)
	h.addTimestamp(ctx, l, &r)
	h.addSeverity(ctx, l, &r)
	h.addSourceLocation(ctx, l, &r)
	h.addTrace(ctx, l, &r)
	h.addLabels(ctx, l, &r)

	err := h.addAttrs(ctx, l, &r)
	err = errors.Join(err, l.End())

	return err
}

// WithAttrs implements slog.Handler.
func (h *Handler) WithAttrs(as []slog.Attr) slog.Handler {
	clone := *h
	staticFields, w := goldjson.NewStaticFields()
	var err error
	for _, attr := range as {
		err = errors.Join(err, h.addAttr(w, attr))
	}
	clone.attrBuilders = cloneAppend(
		h.attrBuilders,
		func(ctx context.Context, h *Handler, l *goldjson.LineWriter, next func(context.Context) error) error {
			l.AddStaticFields(staticFields)
			return errors.Join(err, next(ctx))
		},
	)
	err = w.End()
	return &clone
}

// WithGroup implements slog.Handler.
func (h *Handler) WithGroup(name string) slog.Handler {
	clone := *h
	clone.encoder = h.encoder.Clone()
	clone.encoder.PrepareKey(name)
	clone.attrBuilders = cloneAppend(
		h.attrBuilders,
		func(ctx context.Context, h *Handler, l *goldjson.LineWriter, next func(context.Context) error) error {
			l.StartRecord(name)
			defer l.EndRecord()
			return next(ctx)
		},
	)
	return &clone
}

// Enabled implements slog.Handler.
func (h *Handler) Enabled(ctx context.Context, l slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.config.Level != nil {
		minLevel = h.config.Level.Level()
	}
	return l >= minLevel
}

func (h *Handler) addMessage(ctx context.Context, l *goldjson.LineWriter, r *slog.Record) {
	l.AddString(fieldMessage, r.Message)
}

func (h *Handler) addTimestamp(ctx context.Context, l *goldjson.LineWriter, r *slog.Record) {
	time := r.Time.Round(0) // strip monotonic to match Attr behavior
	l.AddTime(fieldTimestamp, time)
}

func (h *Handler) addSeverity(ctx context.Context, l *goldjson.LineWriter, r *slog.Record) {
	switch {
	case r.Level >= slog.LevelError:
		l.AddString(fieldSeverity, "ERROR")
	case r.Level >= slog.LevelWarn:
		l.AddString(fieldSeverity, "WARN")
	case r.Level >= slog.LevelInfo:
		l.AddString(fieldSeverity, "INFO")
	default:
		l.AddString(fieldSeverity, "DEBUG")
	}
}

func (h *Handler) addSourceLocation(ctx context.Context, l *goldjson.LineWriter, r *slog.Record) {
	fs := runtime.CallersFrames([]uintptr{r.PC})
	f, _ := fs.Next()

	l.StartRecord(fieldSourceLocation)
	defer l.EndRecord()

	l.AddString(fieldSourceFile, f.File)
	l.AddInt64(fieldSourceLine, int64(f.Line))
	l.AddString(fieldSourceFunction, f.Function)
}

func (h *Handler) addTrace(ctx context.Context, l *goldjson.LineWriter, r *slog.Record) {
	trace := traceFromContext(ctx)
	if trace.ID == "" {
		return
	}

	l.AddString(fieldTraceID, fmt.Sprintf("projects/%s/traces/%s", h.config.ProjectID, trace.ID))
	if trace.SpanID != "" {
		l.AddString(fieldTraceSpanID, trace.SpanID)
	}
	l.AddBool(fieldTraceSampled, trace.Sampled)
}

func (h *Handler) addLabels(ctx context.Context, l *goldjson.LineWriter, r *slog.Record) {
	opened := false
	labelsFromContext(ctx).Iterate(func(label Label) {
		if !opened {
			opened = true
			l.StartRecord("logging.googleapis.com/labels")
		}
		l.AddString(label.Key, label.Value)
	})
	if opened {
		l.EndRecord()
	}
}

func (h *Handler) addAttrs(ctx context.Context, l *goldjson.LineWriter, r *slog.Record) error {
	if len(h.attrBuilders) == 0 {
		return h.addAttrsRaw(ctx, l, r)
	}

	b := func(ctx context.Context) error {
		return h.addAttrsRaw(ctx, l, r)
	}

	for i := range h.attrBuilders {
		attrBuilder := h.attrBuilders[len(h.attrBuilders)-1-i]
		next := b
		b = func(ctx context.Context) error {
			return attrBuilder(ctx, h, l, next)
		}
	}

	return b(ctx)
}

func (h *Handler) addAttrsRaw(ctx context.Context, l *goldjson.LineWriter, r *slog.Record) error {
	var err error
	r.Attrs(func(attr slog.Attr) bool {
		err = errors.Join(err, h.addAttr(l, attr))
		return true
	})
	return err
}

func (h *Handler) addAttr(l *goldjson.LineWriter, a slog.Attr) error {
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindGroup:
		return h.addGroup(l, a, v)
	case slog.KindString:
		l.AddString(a.Key, v.String())
		return nil
	case slog.KindInt64:
		l.AddInt64(a.Key, v.Int64())
		return nil
	case slog.KindUint64:
		l.AddUint64(a.Key, v.Uint64())
		return nil
	case slog.KindFloat64:
		l.AddFloat64(a.Key, v.Float64())
		return nil
	case slog.KindBool:
		l.AddBool(a.Key, v.Bool())
		return nil
	case slog.KindDuration:
		l.AddInt64(a.Key, int64(v.Duration()))
		return nil
	case slog.KindTime:
		return l.AddTime(a.Key, v.Time())
	case slog.KindAny:
		return h.addAny(l, a, v)
	}
	return fmt.Errorf("bad kind: %s", v.Kind())
}

func (h *Handler) addGroup(l *goldjson.LineWriter, a slog.Attr, v slog.Value) error {
	attrs := v.Group()
	if len(attrs) == 0 {
		return nil
	}
	l.StartRecord(a.Key)
	defer l.EndRecord()
	var err error
	for _, a := range attrs {
		err = errors.Join(err, h.addAttr(l, a))
	}
	return err
}

func (h *Handler) addAny(l *goldjson.LineWriter, a slog.Attr, v slog.Value) error {
	val := v.Any()
	_, jm := val.(json.Marshaler)
	if err, ok := val.(error); ok && !jm {
		l.AddString(a.Key, err.Error())
		return nil
	}
	return l.AddMarshal(a.Key, val)
}

const (
	fieldMessage        = "message"
	fieldTimestamp      = "timestamp"
	fieldSeverity       = "severity"
	fieldSourceLocation = "logging.googleapis.com/sourceLocation"
	fieldSourceFile     = "file"
	fieldSourceLine     = "line"
	fieldSourceFunction = "function"
	fieldTraceID        = "logging.googleapis.com/trace"
	fieldTraceSpanID    = "logging.googleapis.com/spanId"
	fieldTraceSampled   = "logging.googleapis.com/trace_sampled"
	fieldLabels         = "logging.googleapis.com/labels"
)

func cloneSlice[T any](slice []T, extraCap int) []T {
	return append(make([]T, 0, len(slice)+extraCap), slice...)
}

func cloneAppend[T any](slice []T, values ...T) []T {
	return append(cloneSlice(slice, len(values)), values...)
}
