package logging

import (
	"bytes"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

type LogMiddleware struct {
	logger *zerolog.Logger
}

func NewLogMiddleware(logger *zerolog.Logger) *LogMiddleware {
	return &LogMiddleware{
		logger: logger,
	}
}

func (m *LogMiddleware) Func() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			logRespWriter := NewLogResponseWriter(w)
			next.ServeHTTP(logRespWriter, r)

			m.logger.Printf(
				"duration=%s status=%d",
				time.Since(startTime).String(),
				logRespWriter.statusCode)
		})
	}
}

type LogResponseWriter struct {
	http.ResponseWriter
	statusCode int
	buf        bytes.Buffer
}

func NewLogResponseWriter(w http.ResponseWriter) *LogResponseWriter {
	return &LogResponseWriter{ResponseWriter: w}
}

func (w *LogResponseWriter) Write(body []byte) (int, error) {
	w.buf.Write(body)
	return w.ResponseWriter.Write(body)
}
