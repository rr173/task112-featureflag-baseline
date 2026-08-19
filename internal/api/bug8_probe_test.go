package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"task112-featureflag/internal/model"
	"task112-featureflag/internal/store"
)

func TestStatsRestoresEvaluationCountAfterRestart(t *testing.T) {
	db := filepath.Join(t.TempDir(), "restart.db")
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	flag := model.Flag{
		Key: "restart-count", Name: "restart-count", DefaultVariant: "off",
		Variants: []model.Variant{{Key: "off", Value: "0"}, {Key: "on", Value: "1"}},
		Enabled: true, RolloutPercent: 100,
	}
	if err := st.CreateFlag(&flag); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordEvaluationResult(flag.Key, "u1", model.EvalResult{FlagKey: flag.Key, VariantKey: "on", Reason: model.ReasonRollout}); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	st, err = store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	ts := httptest.NewServer(NewServer(st, testAdminToken).Handler())
	t.Cleanup(ts.Close)
	resp, data := doJSON(t, "GET", ts.URL+"/stats", "", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("stats status %d", resp.StatusCode)
	}
	var stats model.Stats
	if err := json.Unmarshal(data, &stats); err != nil {
		t.Fatal(err)
	}
	if stats.EvaluationCount != 1 {
		t.Fatalf("evaluation_count = %d, want 1", stats.EvaluationCount)
	}
}
