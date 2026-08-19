package api

import (
	"encoding/json"
	"testing"
)

func TestEvaluationReportCanonicalizesEmptyTargetIdentity(t *testing.T) {
	ts, _ := newTestServer(t)
	createFlag(t, ts)
	resp, _ := doJSON(t, "POST", ts.URL+"/evaluate", "", map[string]any{
		"flag_key":   "dark_mode",
		"target_key": "",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("evaluate status %d", resp.StatusCode)
	}
	resp, data := doJSON(t, "GET", ts.URL+"/evaluation-report?flag_key=dark_mode", "", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("report status %d", resp.StatusCode)
	}
	var out struct {
		Evaluations []struct {
			Identity string `json:"Identity"`
		} `json:"evaluations"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Evaluations) != 1 || out.Evaluations[0].Identity != "anonymous" {
		t.Fatalf("report identity = %+v, want anonymous", out.Evaluations)
	}
}
