package eval

import (
	"testing"

	"task112-featureflag/internal/model"
	"task112-featureflag/internal/segment"
)

func lookupFrom(segs ...*model.Segment) SegmentLookup {
	m := map[string]*model.Segment{}
	for _, s := range segs {
		m[s.ID] = s
	}
	return func(id string) (*model.Segment, bool) { s, ok := m[id]; return s, ok }
}

func baseFlag() *model.Flag {
	return &model.Flag{
		Key:            "ff",
		Enabled:        true,
		DefaultVariant: "control",
		Variants:       []model.Variant{{Key: "control", Value: "old"}, {Key: "treatment", Value: "new"}},
		RolloutPercent: 0,
	}
}

func TestEvaluateDisabled(t *testing.T) {
	f := baseFlag()
	f.Enabled = false
	r := Evaluate(f, lookupFrom(), model.EvalContext{TargetKey: "x"})
	if r.Reason != model.ReasonDisabled || r.VariantKey != "control" {
		t.Fatalf("disabled: %+v", r)
	}
}

func TestEvaluateRuleMatch(t *testing.T) {
	f := baseFlag()
	f.Rules = []model.Rule{{ID: "r1", Priority: 1, SegmentIDs: []string{"vip"}, Op: "matchAny", VariantKey: "treatment"}}
	seg := &model.Segment{ID: "vip", Members: []string{"u1"}}
	r := Evaluate(f, lookupFrom(seg), model.EvalContext{TargetKey: "u1"})
	if r.Reason != model.ReasonRule || r.VariantKey != "treatment" {
		t.Fatalf("rule match: %+v", r)
	}
}

func TestEvaluateRuleNoMatchFallsToDefault(t *testing.T) {
	f := baseFlag()
	f.Rules = []model.Rule{{ID: "r1", Priority: 1, SegmentIDs: []string{"vip"}, Op: "matchAny", VariantKey: "treatment"}}
	seg := &model.Segment{ID: "vip", Members: []string{"other"}}
	r := Evaluate(f, lookupFrom(seg), model.EvalContext{TargetKey: "u2"})
	if r.Reason != model.ReasonDefault || r.VariantKey != "control" {
		t.Fatalf("expected default: %+v", r)
	}
}

func TestEvaluateRolloutFull(t *testing.T) {
	f := baseFlag()
	f.RolloutPercent = 100
	r := Evaluate(f, lookupFrom(), model.EvalContext{TargetKey: "anyone"})
	if r.Reason != model.ReasonRollout || r.VariantKey != "treatment" {
		t.Fatalf("rollout=100: %+v", r)
	}
}

func TestEvaluateRulePriorityOrder(t *testing.T) {
	f := baseFlag()
	f.Rules = []model.Rule{
		{ID: "low", Priority: 10, SegmentIDs: []string{"all"}, Op: "matchAny", VariantKey: "control"},
		{ID: "high", Priority: 1, SegmentIDs: []string{"all"}, Op: "matchAny", VariantKey: "treatment"},
	}
	seg := &model.Segment{ID: "all", Members: []string{"u"}}
	r := Evaluate(f, lookupFrom(seg), model.EvalContext{TargetKey: "u"})
	if r.VariantKey != "treatment" {
		t.Fatalf("expected high-priority rule, got %+v", r)
	}
}

func TestSegmentMatchRuleAND(t *testing.T) {
	seg := &model.Segment{
		ID:      "premium_us",
		Members: []string{},
		Rules: []model.SegmentRule{
			{Attribute: "country", Op: "eq", Value: "US"},
			{Attribute: "plan", Op: "eq", Value: "premium"},
		},
	}
	if !segment.MatchSegment(seg, model.EvalContext{Attributes: map[string]string{"country": "US", "plan": "premium"}}) {
		t.Fatal("expected match")
	}
	if segment.MatchSegment(seg, model.EvalContext{Attributes: map[string]string{"country": "US", "plan": "free"}}) {
		t.Fatal("expected no match (AND)")
	}
}

func TestSegmentMatchMember(t *testing.T) {
	seg := &model.Segment{ID: "m", Members: []string{"u9"}}
	if !segment.MatchSegment(seg, model.EvalContext{TargetKey: "u9"}) {
		t.Fatal("expected member match")
	}
}
