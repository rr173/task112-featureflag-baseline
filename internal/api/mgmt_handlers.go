package api

import (
	"net/http"

	"task112-featureflag/internal/model"
)

func (s *Server) handleGetFlagHistory(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	history, err := s.store.GetFlagHistory(key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"flag_key": key, "history": history})
}

func (s *Server) handleBulkCreateFlags(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	var body struct {
		Flags []model.Flag `json:"flags"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	created := 0
	var errs []string
	for i := range body.Flags {
		if err := s.store.CreateFlag(&body.Flags[i]); err != nil {
			errs = append(errs, body.Flags[i].Key+": "+err.Error())
			continue
		}
		created++
		_ = s.store.AppendAudit(model.AuditEvent{Action: "create_flag", FlagKey: body.Flags[i].Key, Actor: actor})
	}
	writeJSON(w, http.StatusOK, map[string]any{"created": created, "errors": errs})
}

func (s *Server) handleSetTags(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	key := r.PathValue("key")
	var body struct {
		Tags []string `json:"tags"`
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
	f.Tags = model.UniqueStrings(body.Tags)
	if err := s.store.UpdateFlag(f); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.AppendAudit(model.AuditEvent{Action: "set_tags", FlagKey: key, Actor: actor, Detail: joinStrs(body.Tags)})
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	counts := map[string]int{}
	for _, f := range s.store.ListFlags() {
		for _, t := range f.Tags {
			counts[t]++
		}
	}
	tags := make([]map[string]any, 0, len(counts))
	for t, c := range counts {
		tags = append(tags, map[string]any{"tag": t, "count": c})
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": tags})
}

// handleGetDependencies 返回开关的传递性前置依赖闭包，并标注是否存在被停用或缺失的依赖。
func (s *Server) handleGetDependencies(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	deps, disabled, missing, cycle := s.store.DependencyClosure(key)
	if cycle && len(missing) == 0 {
		cycle = true
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"flag_key":     key,
		"dependencies": deps,
		"disabled":     disabled,
		"missing":      missing,
		"cycle":        cycle,
	})
}

// handleCopyFlag 以现有开关为模板复制出一个新开关（key 由请求指定）。
func (s *Server) handleCopyFlag(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	srcKey := r.PathValue("key")
	var body struct {
		NewKey string `json:"new_key"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if body.NewKey == "" {
		writeError(w, http.StatusBadRequest, "new_key is required")
		return
	}
	src, ok := s.store.GetFlag(srcKey)
	if !ok {
		writeError(w, http.StatusNotFound, "flag not found: "+srcKey)
		return
	}
	cp := src.Clone()
	cp.Key = body.NewKey
	cp.Version = 0
	cp.CreatedAt = 0
	cp.UpdatedAt = 0
	if err := s.store.CreateFlag(cp); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.AppendAudit(model.AuditEvent{Action: "copy_flag", FlagKey: body.NewKey, Actor: actor, Detail: srcKey})
	writeJSON(w, http.StatusCreated, cp)
}

// handleCopySegment 以现有分群为模板复制出一个新分群（id 由请求指定）。
func (s *Server) handleCopySegment(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	srcID := r.PathValue("id")
	var body struct {
		NewID string `json:"new_id"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if body.NewID == "" {
		writeError(w, http.StatusBadRequest, "new_id is required")
		return
	}
	src, ok := s.store.GetSegment(srcID)
	if !ok {
		writeError(w, http.StatusNotFound, "segment not found: "+srcID)
		return
	}
	cp := src.Clone()
	cp.ID = body.NewID
	cp.CreatedAt = 0
	cp.UpdatedAt = 0
	if err := s.store.CreateSegment(cp); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.AppendAudit(model.AuditEvent{Action: "copy_segment", Actor: actor, Detail: srcID + "->" + body.NewID})
	writeJSON(w, http.StatusCreated, cp)
}
