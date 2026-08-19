package api

import (
	"net/http"
	"strconv"

	"task112-featureflag/internal/eval"
	"task112-featureflag/internal/evalreport"
	"task112-featureflag/internal/model"
)

func (s *Server) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FlagKey    string            `json:"flag_key"`
		TargetKey  string            `json:"target_key"`
		Attributes map[string]string `json:"attributes"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	res := s.evaluate(body.FlagKey, body.TargetKey, body.Attributes)
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleEvaluateGet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	flagKey := q.Get("flag_key")
	targetKey := q.Get("target_key")
	attrs := map[string]string{}
	for k := range q {
		if k == "flag_key" || k == "target_key" {
			continue
		}
		attrs[k] = q.Get(k)
	}
	res := s.evaluate(flagKey, targetKey, attrs)
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) evaluate(flagKey, targetKey string, attrs map[string]string) model.EvalResult {
	f, ok := s.store.GetFlag(flagKey)
	if !ok {
		res := model.NewErrorResult(flagKey)
		_ = s.store.RecordEvaluationResult(flagKey, model.NormalizeTargetKey(targetKey), res)
		return res
	}
	targetKey = model.NormalizeTargetKey(targetKey)
	ctx := model.EvalContext{TargetKey: targetKey, Attributes: attrs}
	// 依赖检查：任一前置开关缺失、停用或其传递依赖未满足，则回落默认变量。
	if !s.prerequisitesSatisfied(f, map[string]bool{}) {
		res := model.EvalResult{
			FlagKey:    flagKey,
			VariantKey: f.DefaultVariant,
			Value:      f.VariantValue(f.DefaultVariant),
			Reason:     model.ReasonPrerequisite,
		}
		_ = s.store.RecordEvaluationResult(flagKey, targetKey, res)
		return res
	}
	res := eval.Evaluate(f, s.segmentLookup, ctx)
	_ = s.store.RecordEvaluationResult(flagKey, targetKey, res)
	return res
}

// prerequisitesSatisfied 报告 flag 的全部前置依赖（含传递依赖）是否均处于可用状态：
// 依赖项必须存在、已启用，且其自身前置依赖亦满足。visiting 记录当前递归路径上的
// 开关 key，用于检测并拒绝环形依赖，避免无限递归；它在回溯时清除标记，故对菱形依赖
// （同一开关经多条路径被依赖）不会误判。
func (s *Server) prerequisitesSatisfied(f *model.Flag, visiting map[string]bool) bool {
	for _, pre := range f.Prerequisites {
		if visiting[pre] {
			// 环形依赖：视为未满足，中止该路径。
			return false
		}
		pf, ok := s.store.GetFlag(pre)
		if !ok || !pf.Enabled {
			return false
		}
		visiting[pre] = true
		sat := s.prerequisitesSatisfied(pf, visiting)
		visiting[pre] = false
		if !sat {
			return false
		}
	}
	return true
}

func (s *Server) handleBatchEvaluate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TargetKey  string            `json:"target_key"`
		Attributes map[string]string `json:"attributes"`
		FlagKeys   []string          `json:"flag_keys"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	results := make([]model.EvalResult, 0, len(body.FlagKeys))
	for _, key := range body.FlagKeys {
		results = append(results, s.evaluate(key, body.TargetKey, body.Attributes))
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"flags":    s.store.ListFlags(),
		"segments": s.store.ListSegments(),
	})
}

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := parseIntSafe(v); err == nil && n > 0 {
			limit = n
		}
	}
	events, err := s.store.ListAudit(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit": events})
}

func (s *Server) handleEvaluationReport(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := parseIntSafe(v); err == nil && n > 0 {
			limit = n
		}
	}
	items, err := s.store.ListEvaluationAudits(limit, r.URL.Query().Get("flag_key"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items = evalreport.SelectRecent(items, limit)
	response := map[string]any{
		"evaluations":  items,
		"enabled_rate": evalreport.EnabledRate(items),
		"summary":      evalreport.BuildSummary(items),
	}
	if flag := r.URL.Query().Get("flag_key"); flag != "" {
		response["flag_summary"] = evalreport.SummarizeFlag(flag, items)
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Stats())
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func parseIntSafe(s string) (int, error) {
	return strconv.Atoi(s)
}
