package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"task112-featureflag/internal/model"
	"task112-featureflag/internal/store"
)

const testAdminToken = "test-admin"

func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	db := filepath.Join(t.TempDir(), "api.db")
	st, err := store.Open(db)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	srv := NewServer(st, testAdminToken)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		ts.Close()
		st.Close()
	})
	return ts, st
}

func doJSON(t *testing.T, method, url, token string, body any) (*http.Response, []byte) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatalf("newreq: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Admin-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, data
}

func createFlag(t *testing.T, ts *httptest.Server) {
	t.Helper()
	f := model.Flag{
		Key:            "dark_mode",
		Name:           "Dark Mode",
		DefaultVariant: "off",
		Variants:       []model.Variant{{Key: "off", Value: "0"}, {Key: "on", Value: "1"}},
		Enabled:        true,
		RolloutPercent: 100,
	}
	resp, _ := doJSON(t, "POST", ts.URL+"/flags", testAdminToken, f)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create flag status %d", resp.StatusCode)
	}
}

func TestCreateAndListFlag(t *testing.T) {
	ts, _ := newTestServer(t)
	createFlag(t, ts)
	resp, data := doJSON(t, "GET", ts.URL+"/flags", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status %d", resp.StatusCode)
	}
	var out struct {
		Flags []model.Flag `json:"flags"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Flags) != 1 || out.Flags[0].Key != "dark_mode" {
		t.Fatalf("unexpected flags: %+v", out.Flags)
	}
}

func TestUnauthorizedWithoutToken(t *testing.T) {
	ts, _ := newTestServer(t)
	resp, _ := doJSON(t, "POST", ts.URL+"/flags", "", model.Flag{Key: "x"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestEvaluateFlow(t *testing.T) {
	ts, _ := newTestServer(t)
	createFlag(t, ts)
	resp, data := doJSON(t, "POST", ts.URL+"/evaluate", "", map[string]any{
		"flag_key":   "dark_mode",
		"target_key": "u1",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("evaluate status %d", resp.StatusCode)
	}
	var res model.EvalResult
	if err := json.Unmarshal(data, &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.Reason != model.ReasonRollout || res.VariantKey != "on" {
		t.Fatalf("unexpected eval: %+v", res)
	}
}

func TestSegmentAndMatch(t *testing.T) {
	ts, _ := newTestServer(t)
	seg := model.Segment{ID: "beta", Name: "Beta", Members: []string{"u_beta"}}
	resp, _ := doJSON(t, "POST", ts.URL+"/segments", testAdminToken, seg)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create segment status %d", resp.StatusCode)
	}
	resp, data := doJSON(t, "POST", ts.URL+"/segments/beta/match", "", model.EvalContext{TargetKey: "u_beta"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("match status %d", resp.StatusCode)
	}
	var out struct {
		Match bool `json:"match"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.Match {
		t.Fatal("expected beta member to match")
	}
}

func TestStats(t *testing.T) {
	ts, _ := newTestServer(t)
	createFlag(t, ts)
	resp, data := doJSON(t, "GET", ts.URL+"/stats", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stats status %d", resp.StatusCode)
	}
	var st model.Stats
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if st.FlagCount != 1 {
		t.Fatalf("unexpected flag count: %d", st.FlagCount)
	}
}

func TestPrerequisites(t *testing.T) {
	ts, _ := newTestServer(t)
	// 基础开关（依赖项），100% 灰度。
	base := model.Flag{
		Key: "base", Name: "Base", DefaultVariant: "off",
		Variants: []model.Variant{{Key: "off", Value: "0"}, {Key: "on", Value: "1"}},
		Enabled:  true, RolloutPercent: 100,
	}
	if resp, _ := doJSON(t, "POST", ts.URL+"/flags", testAdminToken, base); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create base status %d", resp.StatusCode)
	}
	// 依赖开关。
	dep := model.Flag{
		Key: "dep", Name: "Dep", DefaultVariant: "off",
		Variants: []model.Variant{{Key: "off", Value: "0"}, {Key: "on", Value: "1"}},
		Enabled:  true, RolloutPercent: 100, Prerequisites: []string{"base"},
	}
	if resp, _ := doJSON(t, "POST", ts.URL+"/flags", testAdminToken, dep); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create dep status %d", resp.StatusCode)
	}

	evalURL := ts.URL + "/evaluate"
	// base 启用时，dep 命中治疗。
	_, d1 := doJSON(t, "POST", evalURL, "", map[string]any{"flag_key": "dep", "target_key": "u1"})
	var res1 model.EvalResult
	_ = json.Unmarshal(d1, &res1)
	if res1.Reason != model.ReasonRollout || res1.VariantKey != "on" {
		t.Fatalf("dep expected rollout/on, got %+v", res1)
	}

	// 停用 base 后，dep 回落默认（prerequisite）。
	doJSON(t, "POST", ts.URL+"/flags/base/disable", testAdminToken, nil)
	_, d2 := doJSON(t, "POST", evalURL, "", map[string]any{"flag_key": "dep", "target_key": "u1"})
	var res2 model.EvalResult
	_ = json.Unmarshal(d2, &res2)
	if res2.Reason != "prerequisite" || res2.VariantKey != "off" {
		t.Fatalf("dep expected prerequisite/off, got %+v", res2)
	}
}

func TestBatchEvaluate(t *testing.T) {
	ts, _ := newTestServer(t)
	createFlag(t, ts)
	resp, data := doJSON(t, "POST", ts.URL+"/evaluate/batch", "", map[string]any{
		"target_key": "u1",
		"flag_keys":  []string{"dark_mode"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("batch status %d", resp.StatusCode)
	}
	var out struct {
		Results []model.EvalResult `json:"results"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Results) != 1 || out.Results[0].VariantKey != "on" {
		t.Fatalf("unexpected batch results: %+v", out.Results)
	}
}

func TestListFlagsByTag(t *testing.T) {
	ts, _ := newTestServer(t)
	tagged := model.Flag{
		Key: "search", Name: "Search", Tags: []string{"ui", "beta"},
		DefaultVariant: "off", Variants: []model.Variant{{Key: "off", Value: "0"}, {Key: "on", Value: "1"}},
		Enabled: true, RolloutPercent: 100,
	}
	if resp, _ := doJSON(t, "POST", ts.URL+"/flags", testAdminToken, tagged); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create tagged status %d", resp.StatusCode)
	}
	_, data := doJSON(t, "GET", ts.URL+"/flags?tag=beta", "", nil)
	var out struct {
		Flags []model.Flag `json:"flags"`
	}
	_ = json.Unmarshal(data, &out)
	if len(out.Flags) != 1 || out.Flags[0].Key != "search" {
		t.Fatalf("tag filter unexpected: %+v", out.Flags)
	}
}

func TestExport(t *testing.T) {
	ts, _ := newTestServer(t)
	createFlag(t, ts)
	resp, _ := doJSON(t, "GET", ts.URL+"/export", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("export status %d", resp.StatusCode)
	}
}
