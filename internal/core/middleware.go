package core

import (
	"expvar"
	"net/http"
	"strconv"
	"time"

	"github.com/charmbracelet/log"
)

var requests = expvar.NewInt("http_requests_total")

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

type loggingMiddleware struct{}

func (m loggingMiddleware) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := strconv.FormatInt(start.UnixNano(), 36)
		rr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		rr.Header().Set("X-Request-ID", requestID)
		requests.Add(1)
		next.ServeHTTP(rr, r)
		log.Info("request", "id", requestID, "method", r.Method, "path", r.URL.Path, "status", rr.status, "duration", time.Since(start))
	})
}
