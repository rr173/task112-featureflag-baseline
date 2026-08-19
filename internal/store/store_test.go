package store

import (
	"path/filepath"
	"testing"

	"task112-featureflag/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db := filepath.Join(t.TempDir(), "test.db")
	st, err := Open(db)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestFlagCRUD(t *testing.T) {
	st := newTestStore(t)
	f := &model.Flag{
		Key:            "f",
		Name:           "F",
		DefaultVariant: "off",
		Variants:       []model.Variant{{Key: "off", Value: "0"}, {Key: "on", Value: "1"}},
		Enabled:        true,
	}
	if err := st.CreateFlag(f); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, ok := st.GetFlag("f"); !ok {
		t.Fatal("flag not found after create")
	}
	// duplicate
	if err := st.CreateFlag(f); err == nil {
		t.Fatal("expected duplicate error")
	}
	f.Name = "F2"
	if err := st.UpdateFlag(f); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := st.GetFlag("f")
	if got.Name != "F2" || got.Version != 2 {
		t.Fatalf("unexpected after update: %+v", got)
	}
	if err := st.DeleteFlag("f"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := st.GetFlag("f"); ok {
		t.Fatal("flag still present after delete")
	}
}

func TestSegmentCRUDAndMembers(t *testing.T) {
	st := newTestStore(t)
	seg := &model.Segment{ID: "vip", Name: "VIP", Members: []string{"u1"}}
	if err := st.CreateSegment(seg); err != nil {
		t.Fatalf("create segment: %v", err)
	}
	got, _ := st.GetSegment("vip")
	if len(got.Members) != 1 {
		t.Fatalf("unexpected members: %v", got.Members)
	}
}

func TestRestartRecovery(t *testing.T) {
	db := filepath.Join(t.TempDir(), "persist.db")
	st, err := Open(db)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	f := &model.Flag{
		Key:            "persist",
		Name:           "Persist",
		DefaultVariant: "off",
		Variants:       []model.Variant{{Key: "off", Value: "0"}},
	}
	if err := st.CreateFlag(f); err != nil {
		t.Fatalf("create: %v", err)
	}
	_ = st.AppendAudit(model.AuditEvent{Action: "create_flag", FlagKey: "persist"})
	st.Close()

	// 重新打开同一库，数据应完整恢复。
	st2, err := Open(db)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st2.Close()
	if _, ok := st2.GetFlag("persist"); !ok {
		t.Fatal("flag not recovered")
	}
	if st2.Stats().AuditCount != 1 {
		t.Fatalf("audit not recovered: %d", st2.Stats().AuditCount)
	}
}

func TestFlagTagsAndPrereqPersist(t *testing.T) {
	db := filepath.Join(t.TempDir(), "tags.db")
	st, err := Open(db)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	base := &model.Flag{Key: "b", Name: "B", DefaultVariant: "off", Variants: []model.Variant{{Key: "off", Value: "0"}}}
	if err := st.CreateFlag(base); err != nil {
		t.Fatalf("create base: %v", err)
	}
	dep := &model.Flag{
		Key: "d", Name: "D", Tags: []string{"x", "y"}, Prerequisites: []string{"b"},
		DefaultVariant: "off", Variants: []model.Variant{{Key: "off", Value: "0"}},
	}
	if err := st.CreateFlag(dep); err != nil {
		t.Fatalf("create dep: %v", err)
	}
	st.Close()

	st2, err := Open(db)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st2.Close()
	got, ok := st2.GetFlag("d")
	if !ok {
		t.Fatal("dep not found")
	}
	if len(got.Tags) != 2 || len(got.Prerequisites) != 1 || got.Prerequisites[0] != "b" {
		t.Fatalf("tags/prereq not persisted: %+v", got)
	}
}

func TestFlagHistoryPersist(t *testing.T) {
	db := filepath.Join(t.TempDir(), "hist.db")
	st, err := Open(db)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	f := &model.Flag{Key: "h", Name: "H", DefaultVariant: "off", Variants: []model.Variant{{Key: "off", Value: "0"}}}
	if err := st.CreateFlag(f); err != nil {
		t.Fatalf("create: %v", err)
	}
	f.RolloutPercent = 30
	if err := st.UpdateFlag(f); err != nil {
		t.Fatalf("update: %v", err)
	}
	hist, err := st.GetFlagHistory("h")
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(hist) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(hist))
	}
	st.Close()
}
