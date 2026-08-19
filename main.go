// Command task112-featureflag 启动特性开关与渐进式发布服务，
// 或在不依赖外部服务的情况下执行内置自检（--smoke-test）。
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"task112-featureflag/internal/api"
	"task112-featureflag/internal/eval"
	"task112-featureflag/internal/model"
	"task112-featureflag/internal/store"
)

func main() {
	var (
		addr       string
		dbPath     string
		adminToken string
		smokeTest  bool
	)
	flag.StringVar(&addr, "addr", ":8080", "HTTP 监听地址")
	flag.StringVar(&dbPath, "db", "featureflag.db", "SQLite 数据库文件路径")
	flag.StringVar(&adminToken, "admin-token", "admin-secret", "管理接口所需的 X-Admin-Token")
	flag.BoolVar(&smokeTest, "smoke-test", false, "执行内置自检后退出（不启动 HTTP 服务）")
	flag.Parse()

	if smokeTest {
		if err := runSmokeTest(); err != nil {
			fmt.Fprintln(os.Stderr, "SMOKE TEST FAILED:", err)
			os.Exit(1)
		}
		fmt.Println("SMOKE TEST PASSED")
		os.Exit(0)
	}

	absDB, err := filepath.Abs(dbPath)
	if err != nil {
		log.Fatalf("resolve db path: %v", err)
	}
	st, err := store.Open(absDB)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	srv := api.NewServer(st, adminToken)
	log.Printf("task112-featureflag listening on %s (db=%s)", addr, absDB)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

// runSmokeTest 在临时 SQLite 上构造开关与分群，并断言求值结果符合预期。
// 全程不依赖外部服务，也不依赖真实时间睡眠。
func runSmokeTest() error {
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("featureflag-smoke-%d.db", os.Getpid()))
	defer os.Remove(tmp)

	st, err := store.Open(tmp)
	if err != nil {
		return fmt.Errorf("open temp store: %w", err)
	}
	defer st.Close()

	// 1) 创建多变量开关：控制变量 control，治疗变量 treatment。
	flag := &model.Flag{
		Key:            "checkout_v2",
		Name:           "Checkout V2",
		DefaultVariant: "control",
		Variants: []model.Variant{
			{Key: "control", Value: "old"},
			{Key: "treatment", Value: "new"},
		},
		Enabled:        true,
		RolloutPercent: 100, // 全部命中治疗
	}
	if err := st.CreateFlag(flag); err != nil {
		return fmt.Errorf("create flag: %w", err)
	}

	// 2) 创建分群：VIP 用户命中。
	seg := &model.Segment{
		ID:      "vip",
		Name:    "VIP",
		Members: []string{"user_vip"},
	}
	if err := st.CreateSegment(seg); err != nil {
		return fmt.Errorf("create segment: %w", err)
	}

	// 3) 给开关加一条定向规则：VIP 永远拿到 control。
	rule := model.Rule{ID: "rule_vip", FlagKey: "checkout_v2", Priority: 1, SegmentIDs: []string{"vip"}, Op: "matchAny", VariantKey: "control"}
	f, _ := st.GetFlag("checkout_v2")
	f.Rules = append(f.Rules, rule)
	if err := st.UpdateFlag(f); err != nil {
		return fmt.Errorf("add rule: %w", err)
	}

	// 断言：VIP 用户经规则命中 control（无视 100% 灰度）。
	vipRes := evalResult(st, "checkout_v2", "user_vip", nil)
	if vipRes.VariantKey != "control" || vipRes.Reason != model.ReasonRule {
		return fmt.Errorf("vip expected control/rule, got %s/%s", vipRes.VariantKey, vipRes.Reason)
	}

	// 断言：普通用户因 100% 灰度命中 treatment。
	normRes := evalResult(st, "checkout_v2", "user_normal", nil)
	if normRes.VariantKey != "treatment" || normRes.Reason != model.ReasonRollout {
		return fmt.Errorf("normal expected treatment/rollout, got %s/%s", normRes.VariantKey, normRes.Reason)
	}

	// 4) 禁用开关后任何用户都应拿到默认变量。
	f2, _ := st.GetFlag("checkout_v2")
	f2.Enabled = false
	if err := st.UpdateFlag(f2); err != nil {
		return fmt.Errorf("disable flag: %w", err)
	}
	disRes := evalResult(st, "checkout_v2", "user_vip", nil)
	if disRes.VariantKey != "control" || disRes.Reason != model.ReasonDisabled {
		return fmt.Errorf("disabled expected control/disabled, got %s/%s", disRes.VariantKey, disRes.Reason)
	}

	// 5) 重启恢复：用新 Store 实例重新打开同一库，数据应完整。
	st.Close()
	st2, err := store.Open(tmp)
	if err != nil {
		return fmt.Errorf("reopen store: %w", err)
	}
	defer st2.Close()
	if _, ok := st2.GetFlag("checkout_v2"); !ok {
		return fmt.Errorf("flag not recovered after restart")
	}
	if _, ok := st2.GetSegment("vip"); !ok {
		return fmt.Errorf("segment not recovered after restart")
	}
	if st2.Stats().FlagCount != 1 {
		return fmt.Errorf("unexpected flag count after restart: %d", st2.Stats().FlagCount)
	}

	return nil
}

// evalResult 用给定存储对开关求值（构造分群查找闭包后委托给 eval 引擎）。
func evalResult(st *store.Store, flagKey, targetKey string, attrs map[string]string) model.EvalResult {
	f, ok := st.GetFlag(flagKey)
	if !ok {
		return model.EvalResult{FlagKey: flagKey, Reason: model.ReasonError}
	}
	lookup := func(id string) (*model.Segment, bool) { return st.GetSegment(id) }
	ctx := model.EvalContext{TargetKey: targetKey, Attributes: attrs}
	return eval.Evaluate(f, lookup, ctx)
}
