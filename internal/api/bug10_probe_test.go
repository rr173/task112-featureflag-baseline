package api

import (
	"encoding/json"
	"testing"
)

func TestMissingFlagEvaluationIsRecordedInAudit(t *testing.T) {
	ts, _ := newTestServer(t)
	resp, _ := doJSON(t, "POST", ts.URL+"/evaluate", "", map[string]any{
		"flag_key":   "missing-flag",
		"target_key": "u1",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("evaluate status %d", resp.StatusCode)
	}
	resp, data := doJSON(t, "GET", ts.URL+"/evaluation-report?flag_key=missing-flag", "", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("report status %d", resp.StatusCode)
	}
	var out struct {
		Summary struct {
			Total    int            `json:"total"`
			ByReason map[string]int `json:"by_reason"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.Summary.Total != 1 || out.Summary.ByReason["error"] != 1 {
		t.Fatalf("summary = %+v, want one error evaluation", out.Summary)
	}
}
