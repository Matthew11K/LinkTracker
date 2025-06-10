package middleware

import (
	"net/http"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/common/metrics"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

type MetricsMiddleware struct {
	serviceName string
}

func NewMetricsMiddleware(serviceName string) *MetricsMiddleware {
	return &MetricsMiddleware{
		serviceName: serviceName,
	}
}

func (m *MetricsMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     200,
		}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)
		metrics.RecordHTTPRequest(
			m.serviceName,
			r.Method,
			r.URL.Path,
			rw.statusCode,
			duration,
		)
	})
}
