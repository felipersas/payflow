package health

import (
	"encoding/json"
	"net/http"
	"sync"
)

type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

type CheckFunc func() CheckResult

type CheckResult struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Error  string `json:"error,omitempty"`
}

type Checker struct {
	checks []CheckFunc
	mu     sync.RWMutex
}

func NewChecker() *Checker {
	return &Checker{}
}

func (c *Checker) AddCheck(fn CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks = append(c.checks, fn)
}

func (c *Checker) Check() ([]CheckResult, Status) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	results := make([]CheckResult, len(c.checks))
	overall := StatusHealthy

	for i, fn := range c.checks {
		results[i] = fn()
		if results[i].Status == StatusUnhealthy {
			overall = StatusUnhealthy
		} else if results[i].Status == StatusDegraded && overall != StatusUnhealthy {
			overall = StatusDegraded
		}
	}
	return results, overall
}

func (c *Checker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		results, overall := c.Check()

		status := http.StatusOK
		if overall == StatusUnhealthy {
			status = http.StatusServiceUnavailable
		} else if overall == StatusDegraded {
			status = http.StatusOK
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  overall,
			"checks":  results,
			"service": r.URL.Query().Get("service"),
		})
	}
}
