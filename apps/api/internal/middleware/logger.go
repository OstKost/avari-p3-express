package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// RequestLogger returns a middleware that logs incoming HTTP requests using slog.
func RequestLogger() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			reqID := middleware.GetReqID(r.Context())

			defer func() {
				duration := time.Since(start)
				status := ww.Status()

				attrs := []any{
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", status),
					slog.Duration("duration", duration),
					slog.String("ip", r.RemoteAddr),
					slog.String("user_agent", r.UserAgent()),
				}

				if reqID != "" {
					attrs = append(attrs, slog.String("request_id", reqID))
				}

				if status >= 500 {
					slog.Error("HTTP Request Error", attrs...)
				} else if status >= 400 {
					slog.Warn("HTTP Client Request Error", attrs...)
				} else {
					slog.Info("HTTP Request", attrs...)
				}
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
