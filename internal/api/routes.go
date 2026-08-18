package api

// registerRoutes 注册全部 HTTP 路由（>=20 个操作）。
func (s *Server) registerRoutes() {
	m := s.mux

	// 开关
	m.HandleFunc("POST /flags", s.handleCreateFlag)
	m.HandleFunc("POST /flags/bulk", s.handleBulkCreateFlags)
	m.HandleFunc("GET /flags", s.handleListFlags)
	m.HandleFunc("GET /flags/{key}", s.handleGetFlag)
	m.HandleFunc("PUT /flags/{key}", s.handleUpdateFlag)
	m.HandleFunc("DELETE /flags/{key}", s.handleDeleteFlag)
	m.HandleFunc("POST /flags/{key}/enable", s.handleEnableFlag)
	m.HandleFunc("POST /flags/{key}/disable", s.handleDisableFlag)
	m.HandleFunc("POST /flags/{key}/rollout", s.handleSetRollout)
	m.HandleFunc("POST /flags/{key}/prerequisites", s.handleSetPrerequisites)
	m.HandleFunc("GET /flags/{key}/history", s.handleGetFlagHistory)
	m.HandleFunc("POST /flags/{key}/tags", s.handleSetTags)
	m.HandleFunc("GET /flags/{key}/dependencies", s.handleGetDependencies)
	m.HandleFunc("POST /flags/{key}/copy", s.handleCopyFlag)
	m.HandleFunc("GET /flags/{key}/rules", s.handleListRules)
	m.HandleFunc("GET /flags/{key}/rules/{ruleId}", s.handleGetRule)
	m.HandleFunc("POST /flags/{key}/rules", s.handleAddRule)
	m.HandleFunc("PUT /flags/{key}/rules/{ruleId}", s.handleUpdateRule)
	m.HandleFunc("DELETE /flags/{key}/rules/{ruleId}", s.handleDeleteRule)

	// 求值
	m.HandleFunc("POST /evaluate", s.handleEvaluate)
	m.HandleFunc("GET /evaluate", s.handleEvaluateGet)
	m.HandleFunc("POST /evaluate/batch", s.handleBatchEvaluate)

	// 分群
	m.HandleFunc("POST /segments", s.handleCreateSegment)
	m.HandleFunc("GET /segments", s.handleListSegments)
	m.HandleFunc("GET /segments/{id}", s.handleGetSegment)
	m.HandleFunc("PUT /segments/{id}", s.handleUpdateSegment)
	m.HandleFunc("DELETE /segments/{id}", s.handleDeleteSegment)
	m.HandleFunc("POST /segments/{id}/copy", s.handleCopySegment)
	m.HandleFunc("GET /tags", s.handleListTags)
	m.HandleFunc("POST /segments/{id}/members", s.handleAddMembers)
	m.HandleFunc("DELETE /segments/{id}/members", s.handleRemoveMembers)
	m.HandleFunc("POST /segments/{id}/match", s.handleMatchSegment)

	// 管理
	m.HandleFunc("GET /audit", s.handleListAudit)
	m.HandleFunc("GET /evaluation-report", s.handleEvaluationReport)
	m.HandleFunc("GET /stats", s.handleStats)
	m.HandleFunc("GET /export", s.handleExport)
	m.HandleFunc("GET /health", s.handleHealth)
}
