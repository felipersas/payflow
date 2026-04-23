package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChecker_NoChecks(t *testing.T) {
	c := NewChecker()
	results, status := c.Check()

	if len(results) != 0 {
		t.Errorf("results length: got %d, want 0", len(results))
	}
	if status != StatusHealthy {
		t.Errorf("status: got %s, want %s", status, StatusHealthy)
	}
}

func TestChecker_AllHealthy(t *testing.T) {
	c := NewChecker()
	c.AddCheck(func() CheckResult {
		return CheckResult{Name: "db", Status: StatusHealthy}
	})

	results, status := c.Check()

	if len(results) != 1 {
		t.Errorf("results length: got %d, want 1", len(results))
	}
	if status != StatusHealthy {
		t.Errorf("status: got %s, want %s", status, StatusHealthy)
	}
	if results[0].Name != "db" {
		t.Errorf("check name: got %s, want db", results[0].Name)
	}
}

func TestChecker_Unhealthy(t *testing.T) {
	c := NewChecker()
	c.AddCheck(func() CheckResult {
		return CheckResult{Name: "db", Status: StatusUnhealthy, Error: "connection failed"}
	})

	results, status := c.Check()

	if len(results) != 1 {
		t.Errorf("results length: got %d, want 1", len(results))
	}
	if status != StatusUnhealthy {
		t.Errorf("status: got %s, want %s", status, StatusUnhealthy)
	}
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

	if len(results) != 2 {
		t.Errorf("results length: got %d, want 2", len(results))
	}
	if status != StatusDegraded {
		t.Errorf("status: got %s, want %s", status, StatusDegraded)
	}
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

	if len(results) != 2 {
		t.Errorf("results length: got %d, want 2", len(results))
	}
	if status != StatusUnhealthy {
		t.Errorf("status: got %s, want %s (unhealthy wins)", status, StatusUnhealthy)
	}
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

	if rr.Code != http.StatusOK {
		t.Errorf("status code: got %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != string(StatusHealthy) {
		t.Errorf("response status: got %v, want %s", resp["status"], StatusHealthy)
	}
	if resp["service"] != "account-service" {
		t.Errorf("response service: got %v, want account-service", resp["service"])
	}
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

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status code: got %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != string(StatusUnhealthy) {
		t.Errorf("response status: got %v, want %s", resp["status"], StatusUnhealthy)
	}
}
