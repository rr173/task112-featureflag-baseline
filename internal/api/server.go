// Package api 暴露特性开关服务的 HTTP 接口。
package api

import (
	"encoding/json"
	"net/http"

	"task112-featureflag/internal/model"
	"task112-featureflag/internal/store"
)

// Server 持有存储与配置，并注册全部 HTTP 路由。
type Server struct {
	store      *store.Store
	adminToken string
	mux        *http.ServeMux
}

// NewServer 创建并初始化服务（注册路由）。
func NewServer(st *store.Store, adminToken string) *Server {
	s := &Server{
		store:      st,
		adminToken: adminToken,
		mux:        http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// Handler 返回可直接用于 http.Server 的 handler。
func (s *Server) Handler() http.Handler { return s.mux }

// ---- 通用辅助 ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}

// requireAdmin 校验 X-Admin-Token 头，返回操作者名称。
func (s *Server) requireAdmin(r *http.Request) (string, bool) {
	token := r.Header.Get("X-Admin-Token")
	if token == "" || token != s.adminToken {
		return "", false
	}
	actor := r.Header.Get("X-Actor")
	return model.NormalizeActor(actor), true
}

func (s *Server) segmentLookup(id string) (*model.Segment, bool) {
	return s.store.GetSegment(id)
}
