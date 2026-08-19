package segment

import (
	"testing"

	"task112-featureflag/internal/model"
)

func TestMissingAttributeDoesNotMatchNeq(t *testing.T) {
	seg := &model.Segment{
		ID: "paid",
		Rules: []model.SegmentRule{{
			Attribute: "plan",
			Op:        "neq",
			Value:     "free",
		}},
	}
	if MatchSegment(seg, model.EvalContext{Attributes: map[string]string{}}) {
		t.Fatal("a missing attribute must not satisfy neq")
	}
}
