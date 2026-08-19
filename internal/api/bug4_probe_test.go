package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"task112-featureflag/internal/model"
)

func modelFlagForRateProbe(key string) model.Flag {
	return model.Flag{
		Key: key, Name: key, DefaultVariant: "off",
		Variants: []model.Variant{{Key: "off", Value: "0"}, {Key: "on", Value: "1"}},
		Enabled: true, RolloutPercent: 100,
	}
}

func createRateProbeFlag(t *testing.T, ts *httptest.Server, flag model.Flag) {
	t.Helper()
	resp, _ := doJSON(t, "POST", ts.URL+"/flags", testAdminToken, flag)
	if resp.StatusCode != 201 {
		t.Fatalf("create %s status %d", flag.Key, resp.StatusCode)
	}
}

func TestPrerequisiteFallbackCountsAsDisabledInReport(t *testing.T) {
	ts, _ := newTestServer(t)
	base := modelFlagForRateProbe("base-rate")
	base.Enabled = false
	createRateProbeFlag(t, ts, base)
	dep := modelFlagForRateProbe("dep-rate")
	dep.Prerequisites = []string{"base-rate"}
	createRateProbeFlag(t, ts, dep)
	doJSON(t, "POST", ts.URL+"/evaluate", "", map[string]any{"flag_key": "dep-rate", "target_key": "u1"})
	resp, data := doJSON(t, "GET", ts.URL+"/evaluation-report?flag_key=dep-rate", "", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("report status %d", resp.StatusCode)
	}
	var out struct {
		EnabledRate float64 `json:"enabled_rate"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.EnabledRate != 0 {
		t.Fatalf("enabled_rate = %v, want 0", out.EnabledRate)
	}
}
