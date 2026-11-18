package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type requestMetric struct {
	count    uint64
	duration float64
}

var requestMetrics = struct {
	sync.Mutex
	values map[string]requestMetric
}{values: make(map[string]requestMetric)}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (recorder *statusRecorder) WriteHeader(status int) {
	if recorder.wroteHeader {
		return
	}
	recorder.status = status
	recorder.wroteHeader = true
	recorder.ResponseWriter.WriteHeader(status)
}

func instrumentHandler(service string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		route := r.URL.Path
		switch route {
		case "/health", "/validate", "/keys":
		default:
			route = "unmatched"
		}
		key := fmt.Sprintf("%s|%s|%s|%d", service, normalizeMethod(r.Method), route, recorder.status)
		requestMetrics.Lock()
		metric := requestMetrics.values[key]
		metric.count++
		metric.duration += time.Since(start).Seconds()
		requestMetrics.values[key] = metric
		requestMetrics.Unlock()
	})
}

func normalizeMethod(method string) string {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
		return method
	default:
		return "OTHER"
	}
}

func metricsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	requestMetrics.Lock()
	defer requestMetrics.Unlock()

	if _, err := fmt.Fprintln(w, "# HELP http_requests_total Total number of HTTP requests."); err != nil {
		return
	}
	if _, err := fmt.Fprintln(w, "# TYPE http_requests_total counter"); err != nil {
		return
	}
	if _, err := fmt.Fprintln(w, "# HELP http_request_duration_seconds Duration of HTTP requests in seconds."); err != nil {
		return
	}
	if _, err := fmt.Fprintln(w, "# TYPE http_request_duration_seconds summary"); err != nil {
		return
	}
	for key, metric := range requestMetrics.values {
		parts := strings.Split(key, "|")
		labels := fmt.Sprintf("service=\"%s\",method=\"%s\",route=\"%s\",status=\"%s\"", parts[0], parts[1], parts[2], parts[3])
		if _, err := fmt.Fprintf(w, "http_requests_total{%s} %d\n", labels, metric.count); err != nil {
			return
		}
		if _, err := fmt.Fprintf(w, "http_request_duration_seconds_count{%s} %d\n", labels, metric.count); err != nil {
			return
		}
		if _, err := fmt.Fprintf(w, "http_request_duration_seconds_sum{%s} %g\n", labels, metric.duration); err != nil {
			return
		}
	}
}
