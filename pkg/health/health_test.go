package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChecker_NoChecks(t *testing.T) {
	c := NewChecker()
	results, status := c.Check()

	assert.Empty(t, results, "results should be empty")
	assert.Equal(t, StatusHealthy, status, "status should be healthy")
}

func TestChecker_AllHealthy(t *testing.T) {
	c := NewChecker()
	c.AddCheck(func() CheckResult {
		return CheckResult{Name: "db", Status: StatusHealthy}
	})

	results, status := c.Check()

	assert.Len(t, results, 1, "results length mismatch")
	assert.Equal(t, StatusHealthy, status, "status should be healthy")
	assert.Equal(t, "db", results[0].Name, "check name mismatch")
}

func TestChecker_Unhealthy(t *testing.T) {
	c := NewChecker()
	c.AddCheck(func() CheckResult {
		return CheckResult{Name: "db", Status: StatusUnhealthy, Error: "connection failed"}
	})

	results, status := c.Check()

	assert.Len(t, results, 1, "results length mismatch")
	assert.Equal(t, StatusUnhealthy, status, "status should be unhealthy")
}

func TestChecker_Mixed(t *testing.T) {
	c := NewChecker()
	c.AddCheck(func() CheckResult {
		return CheckResult{Name: "cache", Status: StatusDegraded}
	})
	c.AddCheck(func() CheckResult {
		return CheckResult{Name: "db", Status: StatusHealthy}
	})

	results, status := c.Check()

	assert.Len(t, results, 2, "results length mismatch")
	assert.Equal(t, StatusDegraded, status, "status should be degraded")
}

func TestChecker_HealthyAndUnhealthy(t *testing.T) {
	c := NewChecker()
	c.AddCheck(func() CheckResult {
		return CheckResult{Name: "cache", Status: StatusHealthy}
	})
	c.AddCheck(func() CheckResult {
		return CheckResult{Name: "db", Status: StatusUnhealthy, Error: "down"}
	})

	results, status := c.Check()

	assert.Len(t, results, 2, "results length mismatch")
	assert.Equal(t, StatusUnhealthy, status, "unhealthy should win")
}

func TestHandler_Healthy(t *testing.T) {
	c := NewChecker()
	c.AddCheck(func() CheckResult {
		return CheckResult{Name: "db", Status: StatusHealthy}
	})

	handler := c.Handler()
	req := httptest.NewRequest("GET", "/health?service=account-service", nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "status code mismatch")

	var resp map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err, "failed to decode response")

	assert.Equal(t, string(StatusHealthy), resp["status"], "response status mismatch")
	assert.Equal(t, "account-service", resp["service"], "response service mismatch")
}

func TestHandler_Unhealthy(t *testing.T) {
	c := NewChecker()
	c.AddCheck(func() CheckResult {
		return CheckResult{Name: "db", Status: StatusUnhealthy, Error: "down"}
	})

	handler := c.Handler()
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code, "status code mismatch")

	var resp map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err, "failed to decode response")

	assert.Equal(t, string(StatusUnhealthy), resp["status"], "response status mismatch")
}
