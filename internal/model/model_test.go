package model

import "testing"

func TestFlagValidate(t *testing.T) {
	good := &Flag{
		Key:            "f1",
		Name:           "Flag One",
		DefaultVariant: "off",
		Variants:       []Variant{{Key: "off", Value: "0"}, {Key: "on", Value: "1"}},
		RolloutPercent: 50,
	}
	if err := good.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}

	badKey := *good
	badKey.Key = "bad key!"
	if err := badKey.Validate(); err == nil {
		t.Fatal("expected invalid key error")
	}

	dupVariant := *good
	dupVariant.Variants = []Variant{{Key: "a", Value: "1"}, {Key: "a", Value: "2"}}
	if err := dupVariant.Validate(); err == nil {
		t.Fatal("expected duplicate variant error")
	}

	badDefault := *good
	badDefault.DefaultVariant = "missing"
	if err := badDefault.Validate(); err == nil {
		t.Fatal("expected missing default variant error")
	}

	badRollout := *good
	badRollout.RolloutPercent = 150
	if err := badRollout.Validate(); err == nil {
		t.Fatal("expected rollout bounds error")
	}
}

func TestSegmentValidate(t *testing.T) {
	good := &Segment{ID: "s1", Name: "Seg", Rules: []SegmentRule{{Attribute: "country", Op: "eq", Value: "US"}}}
	if err := good.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
	badOp := *good
	badOp.Rules = []SegmentRule{{Attribute: "country", Op: "bogus", Value: "US"}}
	if err := badOp.Validate(); err == nil {
		t.Fatal("expected invalid op error")
	}
}

func TestTreatmentVariantKey(t *testing.T) {
	f := &Flag{
		DefaultVariant: "control",
		Variants:       []Variant{{Key: "control", Value: "0"}, {Key: "treatment", Value: "1"}},
	}
	if got := f.TreatmentVariantKey(); got != "treatment" {
		t.Fatalf("expected treatment, got %s", got)
	}
	// 单变量时回退默认。
	single := &Flag{DefaultVariant: "only", Variants: []Variant{{Key: "only", Value: "x"}}}
	if got := single.TreatmentVariantKey(); got != "only" {
		t.Fatalf("expected only, got %s", got)
	}
}

func TestEvalContextAttribute(t *testing.T) {
	ctx := EvalContext{Attributes: map[string]string{"plan": "pro", "name": ""}}
	if v, ok := ctx.Attribute("plan"); !ok || v != "pro" {
		t.Fatalf("explicit non-empty value: got (%q,%v)", v, ok)
	}
	// 显式空值视为已提供。
	if v, ok := ctx.Attribute("name"); !ok || v != "" {
		t.Fatalf("explicit empty value should be present: got (%q,%v)", v, ok)
	}
	// 缺失属性不得被报告为已提供。
	if _, ok := ctx.Attribute("missing"); ok {
		t.Fatal("missing attribute must not be reported as present")
	}
	if _, ok := (EvalContext{}).Attribute("anything"); ok {
		t.Fatal("missing attribute on nil map must not be reported as present")
	}
}

func TestCloneIsDeep(t *testing.T) {
	f := &Flag{Key: "k", Variants: []Variant{{Key: "a"}}, Rules: []Rule{{ID: "r", SegmentIDs: []string{"s"}}}}
	cp := f.Clone()
	cp.Variants[0].Key = "mutated"
	cp.Rules[0].SegmentIDs[0] = "mutated"
	if f.Variants[0].Key == "mutated" {
		t.Fatal("Clone did not deep-copy Variants")
	}
	if f.Rules[0].SegmentIDs[0] == "mutated" {
		t.Fatal("Clone did not deep-copy Rule.SegmentIDs")
	}
}
