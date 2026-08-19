package segment

import (
	"testing"

	"task112-featureflag/internal/model"
)

func TestMatchMember(t *testing.T) {
	seg := &model.Segment{ID: "s", Members: []string{"u1", "u2"}}
	if !MatchSegment(seg, model.EvalContext{TargetKey: "u1"}) {
		t.Fatal("expected member match")
	}
	if MatchSegment(seg, model.EvalContext{TargetKey: "u3"}) {
		t.Fatal("expected no match for non-member")
	}
}

func TestMatchAttributeOps(t *testing.T) {
	seg := &model.Segment{ID: "s", Rules: []model.SegmentRule{
		{Attribute: "country", Op: "eq", Value: "US"},
	}}
	if !MatchSegment(seg, model.EvalContext{Attributes: map[string]string{"country": "US"}}) {
		t.Fatal("eq should match")
	}
	if MatchSegment(seg, model.EvalContext{Attributes: map[string]string{"country": "CN"}}) {
		t.Fatal("eq should not match")
	}

	contains := &model.Segment{ID: "c", Rules: []model.SegmentRule{{Attribute: "email", Op: "contains", Value: "@corp.com"}}}
	if !MatchSegment(contains, model.EvalContext{Attributes: map[string]string{"email": "a@corp.com"}}) {
		t.Fatal("contains should match")
	}

	in := &model.Segment{ID: "i", Rules: []model.SegmentRule{{Attribute: "tier", Op: "in", Value: "gold,silver"}}}
	if !MatchSegment(in, model.EvalContext{Attributes: map[string]string{"tier": "silver"}}) {
		t.Fatal("in should match")
	}
	if MatchSegment(in, model.EvalContext{Attributes: map[string]string{"tier": "bronze"}}) {
		t.Fatal("in should not match")
	}

	gt := &model.Segment{ID: "g", Rules: []model.SegmentRule{{Attribute: "age", Op: "gt", Value: "18"}}}
	if !MatchSegment(gt, model.EvalContext{Attributes: map[string]string{"age": "21"}}) {
		t.Fatal("gt should match")
	}
	if MatchSegment(gt, model.EvalContext{Attributes: map[string]string{"age": "10"}}) {
		t.Fatal("gt should not match")
	}
}

func TestMatchNeqMissingAttribute(t *testing.T) {
	seg := &model.Segment{ID: "n", Rules: []model.SegmentRule{{Attribute: "plan", Op: "neq", Value: "free"}}}
	// 缺失属性不得满足 neq，否则缺少该属性的目标会被误判为命中。
	if MatchSegment(seg, model.EvalContext{Attributes: map[string]string{}}) {
		t.Fatal("missing attribute must not satisfy neq")
	}
	if MatchSegment(seg, model.EvalContext{Attributes: map[string]string{"other": "x"}}) {
		t.Fatal("missing attribute must not satisfy neq")
	}
	// 显式空值同样视为「已提供」，空不等于 "free" 故命中。
	if !MatchSegment(seg, model.EvalContext{Attributes: map[string]string{"plan": ""}}) {
		t.Fatal("explicit empty value should satisfy neq against non-empty")
	}
	// 实际不等则命中。
	if !MatchSegment(seg, model.EvalContext{Attributes: map[string]string{"plan": "pro"}}) {
		t.Fatal("unequal value should satisfy neq")
	}
	// 实际相等则不命中。
	if MatchSegment(seg, model.EvalContext{Attributes: map[string]string{"plan": "free"}}) {
		t.Fatal("equal value must not satisfy neq")
	}
}

func TestMatchEmptySegment(t *testing.T) {
	seg := &model.Segment{ID: "empty"}
	if MatchSegment(seg, model.EvalContext{TargetKey: "x"}) {
		t.Fatal("empty segment with no members should never match")
	}
}
