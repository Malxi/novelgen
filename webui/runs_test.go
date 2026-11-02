package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestListRunMetricsUsesProjectCLIJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := runMetricsCLI
	defer func() { runMetricsCLI = previous }()

	var gotArgs []string
	runMetricsCLI = func(project string, args ...string) ([]byte, error) {
		if project != "book" {
			t.Fatalf("project = %q, want book", project)
		}
		gotArgs = append([]string(nil), args...)
		return []byte(`[{"run_id":"run-1","status":"completed","events":3}]`), nil
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/runs?limit=5&stuck=true", nil)
	c.Set(projectContextKey, "book")
	listRunMetrics(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := len(gotArgs); got != 5 || gotArgs[0] != "runs" || gotArgs[len(gotArgs)-1] != "--stuck" {
		t.Fatalf("args = %#v", gotArgs)
	}
	var response struct {
		Success bool              `json:"success"`
		Data    []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success || len(response.Data) != 1 {
		t.Fatalf("response = %#v", response)
	}
}

func TestGetRunMetricsRejectsPathRunID(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/runs/bad%2Fid", nil)
	c.Params = gin.Params{{Key: "id", Value: "bad/id"}}
	getRunMetrics(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
