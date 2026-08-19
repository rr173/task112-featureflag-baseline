package store

import (
	"testing"

	"task112-featureflag/internal/clock"
	"task112-featureflag/internal/model"
)

func TestAuditIDsRemainUniqueWithinOneMillisecond(t *testing.T) {
	st := newTestStore(t)
	st.clk = clock.NewFixed(1234)
	if err := st.AppendAudit(model.AuditEvent{Action: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := st.AppendAudit(model.AuditEvent{Action: "second"}); err != nil {
		t.Fatal(err)
	}
	events, err := st.ListAudit(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d audit events, want 2", len(events))
	}
}
