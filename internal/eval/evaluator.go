// Package eval 实现特性开关的求值引擎：按禁用、定向规则、灰度发布、默认变量的
// 顺序对目标上下文求值，全部逻辑为纯函数，便于确定性复现与测试。
package eval

import (
	"crypto/sha256"
	"encoding/binary"
	"sort"

	"task112-featureflag/internal/model"
	"task112-featureflag/internal/segment"
)

// SegmentLookup 按 id 查找分群快照；找不到返回 (nil, false)。
type SegmentLookup func(id string) (*model.Segment, bool)

// Evaluate 对单个开关与目标上下文求值。
// 优先级：禁用 → 定向规则（按优先级）→ 百分比灰度 → 默认变量。
func Evaluate(flag *model.Flag, lookup SegmentLookup, ctx model.EvalContext) model.EvalResult {
	res := model.EvalResult{FlagKey: flag.Key}

	if !flag.Enabled {
		res.VariantKey = flag.DefaultVariant
		res.Value = flag.VariantValue(flag.DefaultVariant)
		res.Reason = model.ReasonDisabled
		return res
	}

	// 1) 定向规则：按优先级升序，命中第一条即返回。
	rules := append([]model.Rule(nil), flag.Rules...)
	sortRulesByPriority(rules)
	for _, r := range rules {
		if matchRule(r, lookup, ctx) {
			res.VariantKey = r.VariantKey
			res.Value = flag.VariantValue(r.VariantKey)
			res.Reason = model.ReasonRule
			return res
		}
	}

	// 2) 百分比灰度：仅当存在非默认「治疗」变量时生效。
	if len(flag.Variants) > 1 {
		treatment := flag.TreatmentVariantKey()
		bucket := rolloutBucket(flag.Key, ctx.TargetKey)
		if bucket < flag.RolloutPercent {
			res.VariantKey = treatment
			res.Value = flag.VariantValue(treatment)
			res.Reason = model.ReasonRollout
			return res
		}
	}

	// 3) 默认变量。
	res.VariantKey = flag.DefaultVariant
	res.Value = flag.VariantValue(flag.DefaultVariant)
	res.Reason = model.ReasonDefault
	return res
}

// matchRule 按规则操作符（matchAny/matchAll）聚合其引用分群的命中结果。
func matchRule(r model.Rule, lookup SegmentLookup, ctx model.EvalContext) bool {
	if len(r.SegmentIDs) == 0 {
		return false
	}
	switch r.Op {
	case "matchAny":
		for _, id := range r.SegmentIDs {
			if seg, ok := lookup(id); ok && segment.MatchSegment(seg, ctx) {
				return true
			}
		}
		return false
	case "matchAll":
		for _, id := range r.SegmentIDs {
			seg, ok := lookup(id)
			if !ok || !segment.MatchSegment(seg, ctx) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// sortRulesByPriority 按优先级升序稳定排序，优先级相同按 ID 字典序保证确定性。
func sortRulesByPriority(rules []model.Rule) {
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].Priority != rules[j].Priority {
			return rules[i].Priority < rules[j].Priority
		}
		return rules[i].ID < rules[j].ID
	})
}

// rolloutBucket 基于 key 与 targetKey 的确定性哈希，映射到 [0,100) 的整数桶。
// 相同输入永远得到相同桶，使灰度结果可复现。
func rolloutBucket(flagKey, targetKey string) int {
	if targetKey == "" {
		targetKey = "anonymous"
	}
	h := sha256.Sum256([]byte(flagKey + ":" + targetKey))
	n := binary.BigEndian.Uint32(h[:4])
	return int(n % 100)
}
