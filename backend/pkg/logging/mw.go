package logging

import (
	archivus_constants "archivus/internal/constants"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type LogMiddleware struct {
	debugLog *zerolog.Logger
	auditLog *zerolog.Logger
	errLog   *zerolog.Logger
}

func NewLogMiddleware() *LogMiddleware {
	return &LogMiddleware{
		debugLog: &DebugLogger,
		auditLog: &AuditLogger,
		errLog:   &Errorlogger,
	}
}

func (m *LogMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		clientIP := extractClientIP(r)

		// --- Raw request log (debug) — captured before the handler runs ---
		rawEv := m.debugLog.Debug().
			Str("type", "raw_request").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("query", r.URL.RawQuery).
			Str("ip", clientIP).
			Str("proto", r.Proto).
			Str("host", r.Host)
		for name, vals := range r.Header {
			rawEv = rawEv.Str("req_hdr_"+strings.ToLower(name), fmt.Sprintf("%v", vals))
		}
		rawEv.Msg("raw request")

		// --- Run handler chain ---
		lrw := newLogResponseWriter(w)

		// Recover from panics, log as 500
		defer func() {
			if rec := recover(); rec != nil {
				stack := string(debug.Stack())
				m.errLog.Error().
					Str("type", "panic").
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Str("ip", clientIP).
					Str("user_id", r.Header.Get(archivus_constants.UserId)).
					Interface("panic", rec).
					Str("stack", stack).
					Msg("panic recovered")
				http.Error(lrw, "internal server error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(lrw, r)

		duration := time.Since(startTime)
		status := lrw.statusCode
		userID := r.Header.Get(archivus_constants.UserId)

		// --- Audit log (every request) ---
		m.auditLog.Info().
			Str("type", "audit").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("ip", clientIP).
			Int("status", status).
			Str("user_id", userID).
			Dur("duration_ms", duration).
			Msg("")

		// --- Detailed error log for 5xx responses ---
		if status >= 500 {
			body := lrw.buf.Bytes()
			if len(body) > 512 {
				body = body[:512]
			}
			// Also log CORS response headers for 5xx to aid debugging
			corsHeaders := map[string]string{}
			for _, h := range []string{
				"Access-Control-Allow-Origin",
				"Access-Control-Allow-Methods",
				"Access-Control-Allow-Headers",
				"Access-Control-Expose-Headers",
			} {
				if v := lrw.Header().Get(h); v != "" {
					corsHeaders[h] = v
				}
			}
			m.errLog.Error().
				Str("type", "server_error").
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("ip", clientIP).
				Str("user_id", userID).
				Int("status", status).
				Dur("duration_ms", duration).
				Str("response_body", string(body)).
				Interface("cors_headers", corsHeaders).
				Str("stack", string(debug.Stack())).
				Msg("internal server error")
		}
	})
}

// extractClientIP returns the real client IP, honouring X-Forwarded-For / X-Real-IP.
func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// logResponseWriter wraps http.ResponseWriter to capture status code and response body.
type logResponseWriter struct {
	http.ResponseWriter
	statusCode int
	buf        bytes.Buffer
}

func newLogResponseWriter(w http.ResponseWriter) *logResponseWriter {
	return &logResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (w *logResponseWriter) Write(body []byte) (int, error) {
	w.buf.Write(body)
	return w.ResponseWriter.Write(body)
}

func (w *logResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
