package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"task112-featureflag/internal/model"
)

func (s *Server) handleCreateFlag(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	var f model.Flag
	if err := readJSON(r, &f); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if err := s.store.CreateFlag(&f); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.AppendAudit(model.AuditEvent{Action: "create_flag", FlagKey: f.Key, Actor: actor, Detail: f.Name})
	writeJSON(w, http.StatusCreated, f)
}

func (s *Server) handleListFlags(w http.ResponseWriter, r *http.Request) {
	flags := s.store.ListFlags()
	if tags := r.URL.Query()["tag"]; len(tags) > 0 {
		filtered := flags[:0]
		for _, f := range flags {
			for _, want := range tags {
				if containsStr(f.Tags, want) {
					filtered = append(filtered, f)
					break
				}
			}
		}
		flags = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"flags": flags})
}

func containsStr(slice []string, v string) bool {
	for _, s := range slice {
		if s == v {
			return true
		}
	}
	return false
}

func (s *Server) handleSetPrerequisites(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	key := r.PathValue("key")
	var body struct {
		Prerequisites []string `json:"prerequisites"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	f, ok := s.store.GetFlag(key)
	if !ok {
		writeError(w, http.StatusNotFound, "flag not found: "+key)
		return
	}
	f.Prerequisites = body.Prerequisites
	if err := f.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.UpdateFlag(f); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.AppendAudit(model.AuditEvent{Action: "set_prerequisites", FlagKey: key, Actor: actor, Detail: joinStrs(body.Prerequisites)})
	writeJSON(w, http.StatusOK, f)
}

func joinStrs(s []string) string {
	return strings.Join(s, ",")
}

func (s *Server) handleGetFlag(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	f, ok := s.store.GetFlag(key)
	if !ok {
		writeError(w, http.StatusNotFound, "flag not found: "+key)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleUpdateFlag(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	key := r.PathValue("key")
	var f model.Flag
	if err := readJSON(r, &f); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	f.Key = key
	if err := s.store.UpdateFlag(&f); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.AppendAudit(model.AuditEvent{Action: "update_flag", FlagKey: key, Actor: actor})
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleDeleteFlag(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	key := r.PathValue("key")
	if err := s.store.DeleteFlag(key); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	_ = s.store.AppendAudit(model.AuditEvent{Action: "delete_flag", FlagKey: key, Actor: actor})
	writeJSON(w, http.StatusOK, map[string]any{"deleted": key})
}

func (s *Server) handleEnableFlag(w http.ResponseWriter, r *http.Request) {
	s.setFlagEnabled(w, r, true)
}

func (s *Server) handleDisableFlag(w http.ResponseWriter, r *http.Request) {
	s.setFlagEnabled(w, r, false)
}

func (s *Server) setFlagEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	key := r.PathValue("key")
	f, ok := s.store.GetFlag(key)
	if !ok {
		writeError(w, http.StatusNotFound, "flag not found: "+key)
		return
	}
	f.Enabled = enabled
	if err := s.store.UpdateFlag(f); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	action := "disable_flag"
	if enabled {
		action = "enable_flag"
	}
	_ = s.store.AppendAudit(model.AuditEvent{Action: action, FlagKey: key, Actor: actor})
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleSetRollout(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	key := r.PathValue("key")
	var body struct {
		Percent int `json:"percent"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if body.Percent < 0 || body.Percent > 100 {
		writeError(w, http.StatusBadRequest, "percent must be in [0,100]")
		return
	}
	f, ok := s.store.GetFlag(key)
	if !ok {
		writeError(w, http.StatusNotFound, "flag not found: "+key)
		return
	}
	f.RolloutPercent = body.Percent
	if err := s.store.UpdateFlag(f); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.AppendAudit(model.AuditEvent{Action: "set_rollout", FlagKey: key, Actor: actor, Detail: strconv.Itoa(body.Percent)})
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	f, ok := s.store.GetFlag(key)
	if !ok {
		writeError(w, http.StatusNotFound, "flag not found: "+key)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": f.Rules})
}

func (s *Server) handleGetRule(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	ruleID := r.PathValue("ruleId")
	f, ok := s.store.GetFlag(key)
	if !ok {
		writeError(w, http.StatusNotFound, "flag not found: "+key)
		return
	}
	for _, rl := range f.Rules {
		if rl.ID == ruleID {
			writeJSON(w, http.StatusOK, rl)
			return
		}
	}
	writeError(w, http.StatusNotFound, "rule not found: "+ruleID)
}

func (s *Server) handleAddRule(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	key := r.PathValue("key")
	var rule model.Rule
	if err := readJSON(r, &rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	rule.FlagKey = key
	if rule.ID == "" {
		rule.ID = fmt.Sprintf("r_%d", time.Now().UnixNano())
	}
	f, ok := s.store.GetFlag(key)
	if !ok {
		writeError(w, http.StatusNotFound, "flag not found: "+key)
		return
	}
	// 校验 variant 属于该 flag。
	valid := false
	for _, v := range f.Variants {
		if v.Key == rule.VariantKey {
			valid = true
			break
		}
	}
	if !valid {
		writeError(w, http.StatusBadRequest, "rule variant_key not found in flag: "+rule.VariantKey)
		return
	}
	f.Rules = append(f.Rules, rule)
	if err := s.store.UpdateFlag(f); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.AppendAudit(model.AuditEvent{Action: "add_rule", FlagKey: key, Actor: actor, Detail: rule.ID})
	writeJSON(w, http.StatusCreated, f)
}

func (s *Server) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	key := r.PathValue("key")
	ruleID := r.PathValue("ruleId")
	var rule model.Rule
	if err := readJSON(r, &rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	f, ok := s.store.GetFlag(key)
	if !ok {
		writeError(w, http.StatusNotFound, "flag not found: "+key)
		return
	}
	idx := -1
	for i, rl := range f.Rules {
		if rl.ID == ruleID {
			idx = i
			break
		}
	}
	if idx < 0 {
		writeError(w, http.StatusNotFound, "rule not found: "+ruleID)
		return
	}
	rule.ID = ruleID
	rule.FlagKey = key
	f.Rules[idx] = rule
	if err := s.store.UpdateFlag(f); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.AppendAudit(model.AuditEvent{Action: "update_rule", FlagKey: key, Actor: actor, Detail: ruleID})
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	key := r.PathValue("key")
	ruleID := r.PathValue("ruleId")
	f, ok := s.store.GetFlag(key)
	if !ok {
		writeError(w, http.StatusNotFound, "flag not found: "+key)
		return
	}
	kept := f.Rules[:0]
	found := false
	for _, rl := range f.Rules {
		if rl.ID == ruleID {
			found = true
			continue
		}
		kept = append(kept, rl)
	}
	if !found {
		writeError(w, http.StatusNotFound, "rule not found: "+ruleID)
		return
	}
	f.Rules = kept
	if err := s.store.UpdateFlag(f); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.AppendAudit(model.AuditEvent{Action: "delete_rule", FlagKey: key, Actor: actor, Detail: ruleID})
	writeJSON(w, http.StatusOK, f)
}
