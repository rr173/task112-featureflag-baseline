// Package model 定义特性开关服务的核心领域模型。
package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Variant 表示一个开关变量（一个可返回的取值）。
type Variant struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}

// Rule 表示一条定向规则：当目标命中指定分群时返回固定变量。
type Rule struct {
	ID         string   `json:"id"`
	FlagKey    string   `json:"flag_key"`
	Priority   int      `json:"priority"`
	SegmentIDs []string `json:"segment_ids"`
	Op         string   `json:"op"` // "matchAny" 命中任一分群, "matchAll" 命中全部分群
	VariantKey string   `json:"variant_key"`
}

// Flag 表示一个特性开关。
type Flag struct {
	Key            string    `json:"key"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	Enabled        bool      `json:"enabled"`
	Tags           []string  `json:"tags,omitempty"`
	Variants       []Variant `json:"variants"`
	DefaultVariant string    `json:"default_variant"` // 变量 key
	RolloutPercent int       `json:"rollout_percent"` // 0-100
	Rules          []Rule    `json:"rules,omitempty"`
	// Prerequisites 为依赖的其它开关 key；任一依赖被停用则该开关回落默认变量。
	Prerequisites []string `json:"prerequisites,omitempty"`
	CreatedAt     int64    `json:"created_at"`
	UpdatedAt     int64    `json:"updated_at"`
	Version       int64    `json:"version"`
}

// SegmentRule 表示分群的一个属性谓词。
type SegmentRule struct {
	Attribute string `json:"attribute"`
	Op        string `json:"op"` // eq, neq, in, contains, gt, lt
	Value     string `json:"value"`
}

// Segment 表示一个目标分群。
type Segment struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Rules       []SegmentRule `json:"rules"`
	Members     []string      `json:"members,omitempty"` // 显式成员 key（如 user id）
	CreatedAt   int64         `json:"created_at"`
	UpdatedAt   int64         `json:"updated_at"`
}

// EvalContext 表示一次求值的目标上下文。
type EvalContext struct {
	TargetKey  string            `json:"target_key"` // 例如 user id
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Attribute returns an attribute and whether it was explicitly supplied.
func (c EvalContext) Attribute(name string) (string, bool) {
	v, ok := c.Attributes[name]
	return v, ok
}

// EvalResult 表示一次求值结果。
type EvalResult struct {
	FlagKey    string `json:"flag_key"`
	VariantKey string `json:"variant_key"`
	Value      string `json:"value"`
	Reason     string `json:"reason"` // default, rule, rollout, disabled, error
}

// AuditEvent 表示一次变更审计事件。
type AuditEvent struct {
	ID      string `json:"id"`
	Ts      int64  `json:"ts"`
	Action  string `json:"action"` // create_flag, update_flag, delete_flag, ...
	FlagKey string `json:"flag_key,omitempty"`
	Actor   string `json:"actor,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// Stats 表示服务统计信息。
type Stats struct {
	FlagCount       int   `json:"flag_count"`
	SegmentCount    int   `json:"segment_count"`
	RuleCount       int   `json:"rule_count"`
	AuditCount      int   `json:"audit_count"`
	EvaluationCount int64 `json:"evaluation_count"`
}

// 求值原因常量。
const (
	ReasonDefault      = "default"
	ReasonRule         = "rule"
	ReasonRollout      = "rollout"
	ReasonDisabled     = "disabled"
	ReasonError        = "error"
	ReasonPrerequisite = "prerequisite"
)

// ErrInvalidKey 表示 key 非法。
var ErrInvalidKey = errors.New("model: key must be non-empty and contain only [a-zA-Z0-9_-]")

// ValidKey 校验 key 是否合法。
func ValidKey(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		if !(r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// Validate 校验 Flag 的字段完整性。
func (f *Flag) Validate() error {
	if !ValidKey(f.Key) {
		return ErrInvalidKey
	}
	if f.Name == "" {
		return errors.New("model: flag name is required")
	}
	if len(f.Variants) == 0 {
		return errors.New("model: flag must have at least one variant")
	}
	keys := map[string]bool{}
	for _, v := range f.Variants {
		if v.Key == "" {
			return errors.New("model: variant key is required")
		}
		if keys[v.Key] {
			return errors.New("model: duplicate variant key: " + v.Key)
		}
		keys[v.Key] = true
	}
	if !keys[f.DefaultVariant] {
		return errors.New("model: default_variant must reference an existing variant key")
	}
	if f.RolloutPercent < 0 || f.RolloutPercent > 100 {
		return errors.New("model: rollout_percent must be in [0,100]")
	}
	for i := range f.Rules {
		r := &f.Rules[i]
		if !keys[r.VariantKey] {
			return errors.New("model: rule variant_key must reference an existing variant key: " + r.VariantKey)
		}
		if r.Op != "matchAny" && r.Op != "matchAll" {
			return errors.New("model: rule op must be matchAny or matchAll")
		}
	}
	pre := map[string]bool{}
	for _, p := range f.Prerequisites {
		if p == f.Key {
			return errors.New("model: flag cannot depend on itself")
		}
		if pre[p] {
			return errors.New("model: duplicate prerequisite: " + p)
		}
		pre[p] = true
	}
	return nil
}

// Validate 校验 Segment 的字段完整性。
func (s *Segment) Validate() error {
	if !ValidKey(s.ID) {
		return ErrInvalidKey
	}
	if s.Name == "" {
		return errors.New("model: segment name is required")
	}
	for _, r := range s.Rules {
		switch r.Op {
		case "eq", "neq", "in", "contains", "gt", "lt":
		default:
			return errors.New("model: invalid segment rule op: " + r.Op)
		}
		if r.Attribute == "" {
			return errors.New("model: segment rule attribute is required")
		}
	}
	return nil
}

// TreatmentVariantKey 返回灰度发布使用的「治疗」变量 key（非默认变量的第一个），
// 若不存在其他变量则返回默认变量 key。
func (f *Flag) TreatmentVariantKey() string {
	for _, v := range f.Variants {
		if v.Key != f.DefaultVariant {
			return v.Key
		}
	}
	return f.DefaultVariant
}

// VariantValue 按 key 返回变量取值。
func (f *Flag) VariantValue(key string) string {
	for _, v := range f.Variants {
		if v.Key == key {
			return v.Value
		}
	}
	return ""
}

// Clone 返回 Flag 的深拷贝（用于并发安全的快照）。
func (f *Flag) Clone() *Flag {
	cp := *f
	cp.Variants = append([]Variant{}, f.Variants...)
	cp.Tags = append([]string{}, f.Tags...)
	cp.Prerequisites = append([]string{}, f.Prerequisites...)
	rules := make([]Rule, len(f.Rules))
	for i := range f.Rules {
		r := f.Rules[i]
		r.SegmentIDs = append([]string(nil), f.Rules[i].SegmentIDs...)
		rules[i] = r
	}
	cp.Rules = rules
	return &cp
}

// Clone 返回 Segment 的深拷贝。
func (s *Segment) Clone() *Segment {
	cp := *s
	cp.Rules = append([]SegmentRule(nil), s.Rules...)
	cp.Members = append([]string(nil), s.Members...)
	return &cp
}

// NowMillis 返回当前毫秒时间戳。
func NowMillis() int64 {
	return time.Now().UnixMilli()
}

// NormalizeActor 规范化操作者名称，空值回退为 anonymous。
func NormalizeActor(actor string) string {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return "anonymous"
	}
	return actor
}

// NormalizeTargetKey gives empty evaluation identities a stable persisted name.
func NormalizeTargetKey(target string) string {
	return target
}

// UniqueStrings removes duplicate values while preserving first-seen order.
func UniqueStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

// NewErrorResult creates the canonical result for a missing flag evaluation.
func NewErrorResult(flagKey string) EvalResult {
	return EvalResult{FlagKey: flagKey, Reason: ReasonError}
}

// AuditID makes generated audit identifiers unique even when the clock does not advance.
func AuditID(ts int64, sequence uint64) string {
	return fmt.Sprintf("aud_%d_%d", ts, sequence)
}

// MarshalJSON 的辅助：把切片序列化为 JSON 文本（供 SQLite 存储）。
func MarshalSlice(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// UnmarshalSlice 从 JSON 文本还原切片（供 SQLite 读取）。
func UnmarshalSlice(text string, out any) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return json.Unmarshal([]byte(text), out)
}
