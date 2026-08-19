package api

import (
	"net/http"

	"task112-featureflag/internal/model"
	"task112-featureflag/internal/segment"
)

func (s *Server) handleCreateSegment(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	var seg model.Segment
	if err := readJSON(r, &seg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if err := s.store.CreateSegment(&seg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.AppendAudit(model.AuditEvent{Action: "create_segment", Actor: actor, Detail: seg.ID})
	writeJSON(w, http.StatusCreated, seg)
}

func (s *Server) handleListSegments(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"segments": s.store.ListSegments()})
}

func (s *Server) handleGetSegment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	seg, ok := s.store.GetSegment(id)
	if !ok {
		writeError(w, http.StatusNotFound, "segment not found: "+id)
		return
	}
	writeJSON(w, http.StatusOK, seg)
}

func (s *Server) handleUpdateSegment(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	id := r.PathValue("id")
	var seg model.Segment
	if err := readJSON(r, &seg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	seg.ID = id
	if err := s.store.UpdateSegment(&seg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.AppendAudit(model.AuditEvent{Action: "update_segment", Actor: actor, Detail: id})
	writeJSON(w, http.StatusOK, seg)
}

func (s *Server) handleDeleteSegment(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	id := r.PathValue("id")
	if err := s.store.DeleteSegment(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	_ = s.store.AppendAudit(model.AuditEvent{Action: "delete_segment", Actor: actor, Detail: id})
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

func (s *Server) handleAddMembers(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	id := r.PathValue("id")
	var body struct {
		Members []string `json:"members"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	seg, ok := s.store.GetSegment(id)
	if !ok {
		writeError(w, http.StatusNotFound, "segment not found: "+id)
		return
	}
	set := map[string]bool{}
	for _, m := range seg.Members {
		set[m] = true
	}
	for _, m := range body.Members {
		if m != "" {
			set[m] = true
		}
	}
	seg.Members = seg.Members[:0]
	for m := range set {
		seg.Members = append(seg.Members, m)
	}
	if err := s.store.UpdateSegment(seg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.AppendAudit(model.AuditEvent{Action: "add_members", Actor: actor, Detail: id})
	writeJSON(w, http.StatusOK, seg)
}

func (s *Server) handleRemoveMembers(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid X-Admin-Token")
		return
	}
	id := r.PathValue("id")
	var body struct {
		Members []string `json:"members"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	seg, ok := s.store.GetSegment(id)
	if !ok {
		writeError(w, http.StatusNotFound, "segment not found: "+id)
		return
	}
	rm := map[string]bool{}
	for _, m := range body.Members {
		rm[m] = true
	}
	kept := seg.Members[:0]
	for _, m := range seg.Members {
		if !rm[m] {
			kept = append(kept, m)
		}
	}
	seg.Members = kept
	if err := s.store.UpdateSegment(seg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.AppendAudit(model.AuditEvent{Action: "remove_members", Actor: actor, Detail: id})
	writeJSON(w, http.StatusOK, seg)
}

func (s *Server) handleMatchSegment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	seg, ok := s.store.GetSegment(id)
	if !ok {
		writeError(w, http.StatusNotFound, "segment not found: "+id)
		return
	}
	var ctx model.EvalContext
	if err := readJSON(r, &ctx); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	matched := segment.MatchSegment(seg, ctx)
	writeJSON(w, http.StatusOK, map[string]any{"match": matched, "segment_id": id, "target_key": ctx.TargetKey})
}
