package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"task112-featureflag/internal/model"
)

func modelFlagForProbe(key string) model.Flag {
	return model.Flag{
		Key: key, Name: key, DefaultVariant: "off",
		Variants: []model.Variant{{Key: "off", Value: "0"}, {Key: "on", Value: "1"}},
		Enabled:  true, RolloutPercent: 100,
	}
}

func createProbeFlag(t *testing.T, ts *httptest.Server, flag model.Flag) {
	t.Helper()
	resp, _ := doJSON(t, "POST", ts.URL+"/flags", testAdminToken, flag)
	if resp.StatusCode != 201 {
		t.Fatalf("create %s status %d", flag.Key, resp.StatusCode)
	}
}

func TestEvaluationHonorsDisabledTransitivePrerequisite(t *testing.T) {
	ts, _ := newTestServer(t)
	base := modelFlagForProbe("base")
	base.Enabled = false
	createProbeFlag(t, ts, base)
	middle := modelFlagForProbe("middle")
	middle.Prerequisites = []string{"base"}
	createProbeFlag(t, ts, middle)
	top := modelFlagForProbe("top")
	top.Prerequisites = []string{"middle"}
	createProbeFlag(t, ts, top)

	resp, data := doJSON(t, "POST", ts.URL+"/evaluate", "", map[string]any{
		"flag_key": "top", "target_key": "u1",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("evaluate status %d", resp.StatusCode)
	}
	var result struct {
		VariantKey string `json:"variant_key"`
		Reason     string `json:"reason"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.VariantKey != "off" || result.Reason != "prerequisite" {
		t.Fatalf("result = %+v, want off/prerequisite", result)
	}
}
