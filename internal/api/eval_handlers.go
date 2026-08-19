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
	// 依赖检查：任一前置开关缺失或停用，则回落默认变量。
	if !s.store.PrerequisitesSatisfied(f.Key) {
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
	items = evalreport.SortAudits(items)
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
