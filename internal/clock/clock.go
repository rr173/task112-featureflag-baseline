// Package clock 提供可注入的时间源，便于在测试与自检中固定时间，
// 避免生产代码直接依赖真实时钟。
package clock

import "time"

// Clock 是时间源抽象。
type Clock interface {
	NowMillis() int64
}

// RealClock 使用系统真实时间。
type RealClock struct{}

// NowMillis 返回当前毫秒时间戳。
func (RealClock) NowMillis() int64 { return time.Now().UnixMilli() }

// FixedClock 返回固定的时间，可被 Advance 向前推进，主要用于测试。
type FixedClock struct {
	t int64
}

// NewFixed 创建一个固定时间为 millis 的时钟。
func NewFixed(millis int64) *FixedClock {
	return &FixedClock{t: millis}
}

// NowMillis 返回当前固定时间。
func (c *FixedClock) NowMillis() int64 { return c.t }

// Advance 将固定时间向前推进 d 毫秒。
func (c *FixedClock) Advance(d int64) { c.t += d }
