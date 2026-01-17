package health

import (
	"context"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type HealthChecker struct {
	db *pgxpool.Pool
}

type HealthStatus struct {
	Status     string         `json:"status"`
	Database   DatabaseHealth `json:"database"`
	Goroutines int            `json:"goroutines"`
	Memory     MemoryStats    `json:"memory"`
}

type MemoryStats struct {
	AllocMB      float64 `json:"alloc_mb"`
	TotalAllocMB float64 `json:"total_alloc_mb"`
	SysMB        float64 `json:"sys_mb"`
	NumGC        uint32  `json:"num_gc"`
}

type DatabaseHealth struct {
	Status       string `json:"status"`
	ResponseTime int64  `json:"response_time_ms"`
}

func NewHealthChecker(db *pgxpool.Pool) *HealthChecker {
	return &HealthChecker{db: db}
}

func (h *HealthChecker) CheckBasic() HealthStatus {
	dbHealth := h.checkDatabase()

	status := "healthy"
	if dbHealth.Status != "healthy" {
		status = "unhealthy"
	}

	// Get runtime stats for goroutine leak detection
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return HealthStatus{
		Status:     status,
		Database:   dbHealth,
		Goroutines: runtime.NumGoroutine(),
		Memory: MemoryStats{
			AllocMB:      float64(memStats.Alloc) / 1024 / 1024,
			TotalAllocMB: float64(memStats.TotalAlloc) / 1024 / 1024,
			SysMB:        float64(memStats.Sys) / 1024 / 1024,
			NumGC:        memStats.NumGC,
		},
	}
}

func (h *HealthChecker) checkDatabase() DatabaseHealth {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	err := h.db.Ping(ctx)
	responseTime := time.Since(start).Milliseconds()

	if err != nil {
		return DatabaseHealth{
			Status:       "unhealthy",
			ResponseTime: responseTime,
		}
	}

	return DatabaseHealth{
		Status:       "healthy",
		ResponseTime: responseTime,
	}
}
