package api

import (
	"encoding/json"
	"testing"
)

func TestTagCountsDeduplicateTagsOnOneFlag(t *testing.T) {
	ts, _ := newTestServer(t)
	createFlag(t, ts)
	resp, _ := doJSON(t, "POST", ts.URL+"/flags/dark_mode/tags", testAdminToken, map[string]any{
		"tags": []string{"beta", "beta"},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("set tags status %d", resp.StatusCode)
	}
	resp, data := doJSON(t, "GET", ts.URL+"/tags", "", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("list tags status %d", resp.StatusCode)
	}
	var out struct {
		Tags []struct {
			Tag   string `json:"tag"`
			Count int    `json:"count"`
		} `json:"tags"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Tags) != 1 || out.Tags[0].Tag != "beta" || out.Tags[0].Count != 1 {
		t.Fatalf("tags = %+v, want beta count 1", out.Tags)
	}
}
