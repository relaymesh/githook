package server

import (
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
	size   int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(body)
	r.size += n
	return n, err
}

func requestLogMiddleware(logger *log.Logger) Middleware {
	if logger == nil {
		logger = log.Default()
	}
	return func(next http.Handler) http.Handler {
		if next == nil {
			return nil
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}
			tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
			if tenantID == "" {
				tenantID = strings.TrimSpace(r.Header.Get("X-Githooks-Tenant-ID"))
			}
			requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
			if requestID == "" {
				requestID = strings.TrimSpace(r.Header.Get("X-Request-ID"))
			}
			remoteIP := r.RemoteAddr
			if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
				remoteIP = host
			}
			logger.Printf("http request method=%s path=%s status=%d duration_ms=%d bytes=%d tenant=%s request_id=%s remote_ip=%s ua=%q", r.Method, r.URL.Path, status, time.Since(start).Milliseconds(), rec.size, tenantID, requestID, remoteIP, r.UserAgent())
		})
	}
}
