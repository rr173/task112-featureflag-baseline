// Package segment 实现分群的成员与属性谓词匹配。
package segment

import (
	"strconv"
	"strings"

	"task112-featureflag/internal/model"
)

// MatchSegment 判断目标上下文是否命中分群。
// 规则：若目标 key 在显式成员列表中则直接命中；否则要求分群的全部属性谓词
// 同时满足（AND）。分群无任何规则且无成员时永不命中。
func MatchSegment(seg *model.Segment, ctx model.EvalContext) bool {
	if seg == nil {
		return false
	}
	for _, m := range seg.Members {
		if m == ctx.TargetKey {
			return true
		}
	}
	if len(seg.Rules) == 0 {
		return false
	}
	for _, r := range seg.Rules {
		if !matchRuleAttr(r, ctx) {
			return false
		}
	}
	return true
}

// matchRuleAttr 按操作符比较上下文属性值与谓词值。
func matchRuleAttr(r model.SegmentRule, ctx model.EvalContext) bool {
	val, present := ctx.Attribute(r.Attribute)
	if !present {
		return false
	}
	switch r.Op {
	case "eq":
		return val == r.Value
	case "neq":
		return val != r.Value
	case "contains":
		return strings.Contains(val, r.Value)
	case "in":
		for _, part := range strings.Split(r.Value, ",") {
			if strings.TrimSpace(part) == val {
				return true
			}
		}
		return false
	case "gt":
		return atoiSafe(val) > atoiSafe(r.Value)
	case "lt":
		return atoiSafe(val) < atoiSafe(r.Value)
	default:
		return false
	}
}

func atoiSafe(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}
