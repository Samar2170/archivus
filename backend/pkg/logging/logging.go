package logging

import (
	"archivus/internal/config"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
)

var Errorlogger zerolog.Logger
var AuditLogger zerolog.Logger
var DebugLogger zerolog.Logger

// dateWriter rotates to a new file each calendar day.
type dateWriter struct {
	mu     sync.Mutex
	dir    string
	prefix string
	date   string
	file   *os.File
}

func newDateWriter(dir, prefix string) *dateWriter {
	return &dateWriter{dir: dir, prefix: prefix}
}

func (w *dateWriter) Write(p []byte) (int, error) {
	today := time.Now().Format("2006-01-02")

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil || w.date != today {
		if w.file != nil {
			w.file.Close()
		}
		name := filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.prefix, today))
		f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return 0, err
		}
		w.file = f
		w.date = today
	}

	return w.file.Write(p)
}

func SetupLogging() {
	dir := config.Config.LogsDir

	Errorlogger = zerolog.New(newDateWriter(dir, "error")).With().Timestamp().Logger()
	AuditLogger = zerolog.New(newDateWriter(dir, "audit")).With().Timestamp().Logger()
	DebugLogger = zerolog.New(newDateWriter(dir, "debug")).Level(zerolog.DebugLevel).With().Timestamp().Logger()
}

func HandleError(err error) error {
	if err != nil {
		Errorlogger.Error().
			Err(err).
			Str("stack", string(debug.Stack())).
			Msg("an error occurred")
	}
	return err
}

// Log returns a logger enriched with trace information from the context.
// If the context has no span, it returns the standard Errorlogger.
func Log(ctx context.Context) *zerolog.Event {
	span := trace.SpanFromContext(ctx)
	logger := Errorlogger.With()
	if span.IsRecording() {
		logger = logger.
			Str("trace_id", span.SpanContext().TraceID().String()).
			Str("span_id", span.SpanContext().SpanID().String())
	}
	l := logger.Logger()
	return l.Info()
}

// LogError returns an error event enriched with trace information.
func LogError(ctx context.Context, err error) *zerolog.Event {
	span := trace.SpanFromContext(ctx)
	logger := Errorlogger.Error().Err(err)
	if span.IsRecording() {
		logger = logger.
			Str("trace_id", span.SpanContext().TraceID().String()).
			Str("span_id", span.SpanContext().SpanID().String())
	}
	return logger
}

// LogErrorWithStack logs an error with a full stack trace and optional trace context.
func LogErrorWithStack(ctx context.Context, err error, msg string) {
	span := trace.SpanFromContext(ctx)
	ev := Errorlogger.Error().
		Err(err).
		Str("stack", string(debug.Stack()))
	if span.IsRecording() {
		ev = ev.
			Str("trace_id", span.SpanContext().TraceID().String()).
			Str("span_id", span.SpanContext().SpanID().String())
	}
	ev.Msg(msg)
}

// LogWith returns a logger event from a specific logger, enriched with trace information from the context.
func LogWith(ctx context.Context, logger zerolog.Logger) *zerolog.Event {
	span := trace.SpanFromContext(ctx)
	l := logger.With()
	if span.IsRecording() {
		l = l.
			Str("trace_id", span.SpanContext().TraceID().String()).
			Str("span_id", span.SpanContext().SpanID().String())
	}
	lg := l.Logger()
	return lg.Info()
}
