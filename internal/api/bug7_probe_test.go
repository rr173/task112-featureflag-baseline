package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"task112-featureflag/internal/model"
)

func modelFlagForDiamondProbe(key string) model.Flag {
	return model.Flag{
		Key: key, Name: key, DefaultVariant: "off",
		Variants: []model.Variant{{Key: "off", Value: "0"}, {Key: "on", Value: "1"}},
		Enabled:  true, RolloutPercent: 100,
	}
}

func createDiamondFlag(t *testing.T, ts *httptest.Server, flag model.Flag) {
	t.Helper()
	resp, _ := doJSON(t, "POST", ts.URL+"/flags", testAdminToken, flag)
	if resp.StatusCode != 201 {
		t.Fatalf("create %s status %d", flag.Key, resp.StatusCode)
	}
}

func TestDependencyDiamondIsNotReportedAsCycle(t *testing.T) {
	ts, _ := newTestServer(t)
	createDiamondFlag(t, ts, modelFlagForDiamondProbe("base-diamond"))
	left := modelFlagForDiamondProbe("left-diamond")
	left.Prerequisites = []string{"base-diamond"}
	createDiamondFlag(t, ts, left)
	right := modelFlagForDiamondProbe("right-diamond")
	right.Prerequisites = []string{"base-diamond"}
	createDiamondFlag(t, ts, right)
	top := modelFlagForDiamondProbe("top-diamond")
	top.Prerequisites = []string{"left-diamond", "right-diamond"}
	createDiamondFlag(t, ts, top)

	resp, data := doJSON(t, "GET", ts.URL+"/flags/top-diamond/dependencies", "", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("dependencies status %d", resp.StatusCode)
	}
	var out struct {
		Dependencies []string `json:"dependencies"`
		Cycle        bool     `json:"cycle"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.Cycle || len(out.Dependencies) != 3 {
		t.Fatalf("dependencies = %+v, want 3 nodes and cycle=false", out)
	}
}

// TestDependencyRealCycleStillDetected 守护修复不要「矫枉过正」：
// 真正的回边（A 依赖 B，B 又依赖 A）仍必须被识别为循环依赖。
func TestDependencyRealCycleStillDetected(t *testing.T) {
	ts, _ := newTestServer(t)
	a := modelFlagForDiamondProbe("cycle-a")
	a.Prerequisites = []string{"cycle-b"}
	createDiamondFlag(t, ts, a)
	b := modelFlagForDiamondProbe("cycle-b")
	b.Prerequisites = []string{"cycle-a"}
	createDiamondFlag(t, ts, b)

	resp, data := doJSON(t, "GET", ts.URL+"/flags/cycle-a/dependencies", "", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("dependencies status %d", resp.StatusCode)
	}
	var out struct {
		Dependencies []string `json:"dependencies"`
		Cycle       bool     `json:"cycle"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Cycle {
		t.Fatalf("dependencies = %+v, want cycle=true for real back edge", out)
	}
}
