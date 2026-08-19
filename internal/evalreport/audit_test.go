package evalreport

import "testing"

// TestSelectRecentNewestFirst locks in the report ordering contract: when a
// caller asks for the N most recent observations, the result is newest-to-oldest
// and preserves insertion recency for observations that share a timestamp.
func TestSelectRecentNewestFirst(t *testing.T) {
	// All three observations share a millisecond timestamp; their arrival
	// order (u3 newest, u1 oldest) is carried by Seq (the store rowid). The
	// input here is deliberately oldest-first to prove SelectRecent does not
	// merely preserve input order on ties.
	audits := []Audit{
		{Flag: "dark_mode", Identity: "u1", Evaluated: 1000, Seq: 1, Enabled: true, Variant: "off", Reason: "default"},
		{Flag: "dark_mode", Identity: "u2", Evaluated: 1000, Seq: 2, Enabled: true, Variant: "off", Reason: "default"},
		{Flag: "dark_mode", Identity: "u3", Evaluated: 1000, Seq: 3, Enabled: true, Variant: "on", Reason: "rollout"},
	}
	got := SelectRecent(audits, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(got))
	}
	if got[0].Identity != "u3" || got[1].Identity != "u2" {
		t.Fatalf("expected newest-first [u3,u2], got [%s,%s]", got[0].Identity, got[1].Identity)
	}
}

// TestSortAuditsNewestToOldest verifies the sort direction across distinct
// timestamps and that the input slice is not mutated.
func TestSortAuditsNewestToOldest(t *testing.T) {
	audits := []Audit{
		{Flag: "f", Identity: "old", Evaluated: 900},
		{Flag: "f", Identity: "new", Evaluated: 2000},
		{Flag: "f", Identity: "mid", Evaluated: 1500},
	}
	got := SortAudits(audits)
	if len(got) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(got))
	}
	if got[0].Identity != "new" || got[1].Identity != "mid" || got[2].Identity != "old" {
		t.Fatalf("expected newest-to-oldest [new,mid,old], got [%s,%s,%s]",
			got[0].Identity, got[1].Identity, got[2].Identity)
	}
	// The caller's slice must be left untouched.
	if audits[0].Identity != "old" {
		t.Fatalf("SortAudits mutated its input: %+v", audits)
	}
}
