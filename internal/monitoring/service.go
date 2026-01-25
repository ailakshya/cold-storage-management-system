package monitoring

import (
	"net/http"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

type MonitoringService struct {
	store *TimescaleStore
}

func NewMonitoringService(store *TimescaleStore) *MonitoringService {
	return &MonitoringService{store: store}
}

// StartCollection starts the background metrics collection
func (s *MonitoringService) StartCollection() {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			s.collectAndSave()
		}
	}()
}

func (s *MonitoringService) collectAndSave() {
	cpuPercents, _ := cpu.Percent(time.Second, false)
	cpu := 0.0
	if len(cpuPercents) > 0 {
		cpu = cpuPercents[0]
	}

	memStats, _ := mem.VirtualMemory()
	diskStats, _ := disk.Usage("/")

	s.store.RecordSystemMetrics(
		cpu,
		memStats.Used,
		memStats.Total,
		diskStats.Used,
		diskStats.Total,
	)
}

// Middleware returns a handler that logs request metrics
func (s *MonitoringService) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap ResponseWriter to capture status code
		ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(ww, r)

		duration := time.Since(start)

		// Exclude static assets and monitoring endpoints from logging
		path := r.URL.Path
		/*
			if len(path) > 7 && path[:8] == "/static/" {
				return
			}
		*/

		s.store.RecordAPIMetric(
			r.Method,
			path,
			ww.status,
			duration,
			r.RemoteAddr,
		)
	})
}

// responseWriter wrapper to capture status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
