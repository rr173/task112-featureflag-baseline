package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task112-featureflag/internal/model"
)

func mkFlag(key string) model.Flag {
	return model.Flag{
		Key: key, Name: key, DefaultVariant: "off",
		Variants: []model.Variant{{Key: "off", Value: "0"}, {Key: "on", Value: "1"}},
		Enabled:  true, RolloutPercent: 100,
	}
}

func TestFlagHistory(t *testing.T) {
	ts, _ := newTestServer(t)
	doJSON(t, "POST", ts.URL+"/flags", testAdminToken, mkFlag("hf"))
	// 更新两次，产生版本 2、3。
	f := mkFlag("hf")
	f.RolloutPercent = 50
	doJSON(t, "PUT", ts.URL+"/flags/hf", testAdminToken, f)
	f.RolloutPercent = 0
	doJSON(t, "PUT", ts.URL+"/flags/hf", testAdminToken, f)

	resp, data := doJSON(t, "GET", ts.URL+"/flags/hf/history", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("history status %d", resp.StatusCode)
	}
	var out struct {
		History []model.Flag `json:"history"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.History) != 3 {
		t.Fatalf("expected 3 history versions, got %d", len(out.History))
	}
	// 历史按版本降序：第一条应为版本 3（rollout=0）。
	if out.History[0].Version != 3 || out.History[0].RolloutPercent != 0 {
		t.Fatalf("unexpected newest history: %+v", out.History[0])
	}
}

func TestBulkCreateFlags(t *testing.T) {
	ts, _ := newTestServer(t)
	resp, data := doJSON(t, "POST", ts.URL+"/flags/bulk", testAdminToken, map[string]any{
		"flags": []model.Flag{mkFlag("b1"), mkFlag("b2")},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bulk status %d", resp.StatusCode)
	}
	var out struct {
		Created int `json:"created"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Created != 2 {
		t.Fatalf("expected 2 created, got %d", out.Created)
	}
}

func TestSetTagsAndList(t *testing.T) {
	ts, _ := newTestServer(t)
	doJSON(t, "POST", ts.URL+"/flags", testAdminToken, mkFlag("tf"))
	resp, _ := doJSON(t, "POST", ts.URL+"/flags/tf/tags", testAdminToken, map[string]any{"tags": []string{"x", "y"}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set tags status %d", resp.StatusCode)
	}
	resp, data := doJSON(t, "GET", ts.URL+"/tags", "", nil)
	var out struct {
		Tags []struct {
			Tag   string `json:"tag"`
			Count int    `json:"count"`
		} `json:"tags"`
	}
	_ = json.Unmarshal(data, &out)
	if len(out.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(out.Tags))
	}
}

func TestDependencies(t *testing.T) {
	ts, _ := newTestServer(t)
	base := mkFlag("base")
	doJSON(t, "POST", ts.URL+"/flags", testAdminToken, base)
	mid := mkFlag("mid")
	mid.Prerequisites = []string{"base"}
	doJSON(t, "POST", ts.URL+"/flags", testAdminToken, mid)
	top := mkFlag("top")
	top.Prerequisites = []string{"mid"}
	doJSON(t, "POST", ts.URL+"/flags", testAdminToken, top)

	resp, data := doJSON(t, "GET", ts.URL+"/flags/top/dependencies", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("deps status %d", resp.StatusCode)
	}
	var out struct {
		Dependencies []string `json:"dependencies"`
		Disabled     []string `json:"disabled"`
		Missing      []string `json:"missing"`
		Cycle        bool     `json:"cycle"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Dependencies) != 2 {
		t.Fatalf("expected closure of 2, got %v", out.Dependencies)
	}
	if len(out.Disabled) != 0 || len(out.Missing) != 0 || out.Cycle {
		t.Fatalf("unexpected dep status: %+v", out)
	}
}

func TestCopyFlag(t *testing.T) {
	ts, _ := newTestServer(t)
	doJSON(t, "POST", ts.URL+"/flags", testAdminToken, mkFlag("src"))
	resp, data := doJSON(t, "POST", ts.URL+"/flags/src/copy", testAdminToken, map[string]any{"new_key": "dst"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("copy flag status %d: %s", resp.StatusCode, data)
	}
	if _, ok := s_lookupFlag(t, ts, "dst"); !ok {
		t.Fatal("copied flag not found")
	}
}

func TestCopySegment(t *testing.T) {
	ts, _ := newTestServer(t)
	doJSON(t, "POST", ts.URL+"/segments", testAdminToken, model.Segment{ID: "s1", Name: "S1", Members: []string{"u1"}})
	resp, _ := doJSON(t, "POST", ts.URL+"/segments/s1/copy", testAdminToken, map[string]any{"new_id": "s2"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("copy segment status %d", resp.StatusCode)
	}
}

func TestGetSingleRule(t *testing.T) {
	ts, _ := newTestServer(t)
	doJSON(t, "POST", ts.URL+"/flags", testAdminToken, mkFlag("rf"))
	doJSON(t, "POST", ts.URL+"/flags/rf/rules", testAdminToken, model.Rule{
		ID: "r1", SegmentIDs: []string{}, Op: "matchAny", VariantKey: "on",
	})
	resp, data := doJSON(t, "GET", ts.URL+"/flags/rf/rules/r1", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get rule status %d: %s", resp.StatusCode, data)
	}
	var r model.Rule
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if r.ID != "r1" {
		t.Fatalf("unexpected rule: %+v", r)
	}
}

// s_lookupFlag 通过 /flags/{key} 校验开关是否存在。
func s_lookupFlag(t *testing.T, ts *httptest.Server, key string) (*model.Flag, bool) {
	t.Helper()
	resp, data := doJSON(t, "GET", ts.URL+"/flags/"+key, "", nil)
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	var f model.Flag
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, false
	}
	return &f, true
}
