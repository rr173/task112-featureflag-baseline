package api

import (
	"encoding/json"
	"testing"
)

func TestEvaluationReportReturnsNewestObservationFirst(t *testing.T) {
	ts, _ := newTestServer(t)
	createFlag(t, ts)
	for _, target := range []string{"u1", "u2", "u3"} {
		doJSON(t, "POST", ts.URL+"/evaluate", "", map[string]any{"flag_key": "dark_mode", "target_key": target})
	}
	resp, data := doJSON(t, "GET", ts.URL+"/evaluation-report?flag_key=dark_mode&limit=2", "", nil)
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
	if len(out.Evaluations) != 2 || out.Evaluations[0].Identity != "u3" || out.Evaluations[1].Identity != "u2" {
		t.Fatalf("evaluations = %+v, want newest-first [u3,u2]", out.Evaluations)
	}
}
