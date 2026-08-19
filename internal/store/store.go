// Package store 负责特性开关与分群的领域状态持久化。
// 内存中以 map 保存开关与分群用于快速求值，所有变更同时写入 SQLite，
// 进程重启后从 SQLite 完整恢复，满足「保存与重启恢复」要求。
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	_ "modernc.org/sqlite"

	"task112-featureflag/internal/clock"
	"task112-featureflag/internal/evalreport"
	"task112-featureflag/internal/model"
)

// Store 是特性开关服务的持久化与内存状态中心。
type Store struct {
	mu       sync.RWMutex
	db       *sql.DB
	flags    map[string]*model.Flag
	segments map[string]*model.Segment
	clk      clock.Clock
	evalMu   sync.Mutex
	evalCnt  int64
	evalSeq  uint64
}

// Open 打开（或创建）SQLite 数据库，建表并加载全部开关与分群到内存。
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := createSchema(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: create schema: %w", err)
	}
	s := &Store{
		db:       db,
		flags:    map[string]*model.Flag{},
		segments: map[string]*model.Segment{},
		clk:      clock.RealClock{},
	}
	if err := s.loadAll(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: load: %w", err)
	}
	return s, nil
}

// loadAll 从 SQLite 将开关与分群恢复到内存（重启恢复）。
func (s *Store) loadAll() error {
	flags, err := s.loadFlags()
	if err != nil {
		return err
	}
	for _, f := range flags {
		s.flags[f.Key] = f
	}
	segs, err := s.loadSegments()
	if err != nil {
		return err
	}
	for _, seg := range segs {
		s.segments[seg.ID] = seg
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM evaluations`).Scan(&s.evalCnt); err != nil {
		return err
	}
	return nil
}

// PrerequisitesSatisfied checks the full prerequisite graph without mistaking a diamond for a cycle.
func (s *Store) PrerequisitesSatisfied(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var walk func(string) bool
	walk = func(cur string) bool {
		if visiting[cur] {
			return false
		}
		if visited[cur] {
			return true
		}
		f, ok := s.flags[cur]
		if !ok || !f.Enabled {
			return false
		}
		visiting[cur] = true
		for _, pre := range f.Prerequisites {
			if !walk(pre) {
				return false
			}
		}
		delete(visiting, cur)
		visited[cur] = true
		return true
	}
	return walk(key)
}

// DependencyClosure returns each transitive prerequisite once and reports real cycles only.
func (s *Store) DependencyClosure(key string) (deps, disabled, missing []string, cycle bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state := map[string]uint8{}
	seen := map[string]bool{}
	var walk func(string)
	walk = func(cur string) {
		if state[cur] == 1 {
			cycle = true
			return
		}
		if state[cur] == 2 {
			return
		}
		f, ok := s.flags[cur]
		if !ok {
			if cur != key && !seen[cur] {
				missing = append(missing, cur)
				seen[cur] = true
			}
			return
		}
		state[cur] = 1
		for _, pre := range f.Prerequisites {
			if !seen[pre] {
				deps = append(deps, pre)
				seen[pre] = true
			}
			if pf, ok := s.flags[pre]; ok && !pf.Enabled && !containsString(disabled, pre) {
				disabled = append(disabled, pre)
			}
			walk(pre)
		}
		state[cur] = 2
	}
	walk(key)
	sort.Strings(deps)
	sort.Strings(disabled)
	sort.Strings(missing)
	return deps, disabled, missing, cycle
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func (s *Store) loadFlags() ([]*model.Flag, error) {
	rows, err := s.db.Query(`SELECT key,name,description,enabled,default_variant,rollout_percent,variants_json,rules_json,tags_json,prerequisites_json,created_at,updated_at,version FROM flags`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Flag
	for rows.Next() {
		var f model.Flag
		var enabled, rollout, created, updated, version int64
		var desc, defVar, variantsJSON, rulesJSON, tagsJSON, prereqJSON string
		if err := rows.Scan(&f.Key, &f.Name, &desc, &enabled, &defVar, &rollout, &variantsJSON, &rulesJSON, &tagsJSON, &prereqJSON, &created, &updated, &version); err != nil {
			return nil, err
		}
		f.Description = desc
		f.Enabled = enabled != 0
		f.DefaultVariant = defVar
		f.RolloutPercent = int(rollout)
		f.CreatedAt = created
		f.UpdatedAt = updated
		f.Version = version
		if err := json.Unmarshal([]byte(variantsJSON), &f.Variants); err != nil {
			return nil, fmt.Errorf("store: unmarshal variants for %s: %w", f.Key, err)
		}
		if err := json.Unmarshal([]byte(rulesJSON), &f.Rules); err != nil {
			return nil, fmt.Errorf("store: unmarshal rules for %s: %w", f.Key, err)
		}
		if err := json.Unmarshal([]byte(tagsJSON), &f.Tags); err != nil {
			return nil, fmt.Errorf("store: unmarshal tags for %s: %w", f.Key, err)
		}
		if err := json.Unmarshal([]byte(prereqJSON), &f.Prerequisites); err != nil {
			return nil, fmt.Errorf("store: unmarshal prerequisites for %s: %w", f.Key, err)
		}
		out = append(out, &f)
	}
	return out, rows.Err()
}

func (s *Store) loadSegments() ([]*model.Segment, error) {
	rows, err := s.db.Query(`SELECT id,name,description,rules_json,members_json,created_at,updated_at FROM segments`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Segment
	for rows.Next() {
		var seg model.Segment
		var desc, rulesJSON, membersJSON string
		var created, updated int64
		if err := rows.Scan(&seg.ID, &seg.Name, &desc, &rulesJSON, &membersJSON, &created, &updated); err != nil {
			return nil, err
		}
		seg.Description = desc
		seg.CreatedAt = created
		seg.UpdatedAt = updated
		if err := json.Unmarshal([]byte(rulesJSON), &seg.Rules); err != nil {
			return nil, fmt.Errorf("store: unmarshal seg rules %s: %w", seg.ID, err)
		}
		if err := json.Unmarshal([]byte(membersJSON), &seg.Members); err != nil {
			return nil, fmt.Errorf("store: unmarshal seg members %s: %w", seg.ID, err)
		}
		out = append(out, &seg)
	}
	return out, rows.Err()
}

// ---- Flag 读写 ----

// GetFlag 返回开关的内存快照。
func (s *Store) GetFlag(key string) (*model.Flag, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.flags[key]
	if !ok {
		return nil, false
	}
	return f.Clone(), true
}

// ListFlags 返回全部开关快照。
func (s *Store) ListFlags() []*model.Flag {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Flag, 0, len(s.flags))
	for _, f := range s.flags {
		out = append(out, f.Clone())
	}
	return out
}

// CreateFlag 创建开关（内存+SQLite 原子写入）。
func (s *Store) CreateFlag(f *model.Flag) error {
	if err := f.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.flags[f.Key]; exists {
		return fmt.Errorf("store: flag already exists: %s", f.Key)
	}
	now := model.NowMillis()
	f.CreatedAt = now
	f.UpdatedAt = now
	f.Version = 1
	variantsJSON, _ := json.Marshal(f.Variants)
	rulesJSON, _ := json.Marshal(f.Rules)
	tagsJSON, _ := json.Marshal(f.Tags)
	prereqJSON, _ := json.Marshal(f.Prerequisites)
	const q = `INSERT INTO flags(key,name,description,enabled,default_variant,rollout_percent,variants_json,rules_json,tags_json,prerequisites_json,created_at,updated_at,version)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`
	if _, err := s.db.Exec(q, f.Key, f.Name, f.Description, boolToInt(f.Enabled), f.DefaultVariant, f.RolloutPercent, string(variantsJSON), string(rulesJSON), string(tagsJSON), string(prereqJSON), f.CreatedAt, f.UpdatedAt, f.Version); err != nil {
		return fmt.Errorf("store: insert flag: %w", err)
	}
	s.flags[f.Key] = f.Clone()
	s.recordHistory(f)
	return nil
}

// UpdateFlag 更新已存在开关（按 key 覆盖可变字段，version 自增）。
func (s *Store) UpdateFlag(f *model.Flag) error {
	if err := f.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.flags[f.Key]
	if !ok {
		return fmt.Errorf("store: flag not found: %s", f.Key)
	}
	now := model.NowMillis()
	f.CreatedAt = cur.CreatedAt
	f.Version = cur.Version + 1
	f.UpdatedAt = now
	variantsJSON, _ := json.Marshal(f.Variants)
	rulesJSON, _ := json.Marshal(f.Rules)
	tagsJSON, _ := json.Marshal(f.Tags)
	prereqJSON, _ := json.Marshal(f.Prerequisites)
	const q = `UPDATE flags SET name=?,description=?,enabled=?,default_variant=?,rollout_percent=?,variants_json=?,rules_json=?,tags_json=?,prerequisites_json=?,updated_at=?,version=? WHERE key=?`
	if _, err := s.db.Exec(q, f.Name, f.Description, boolToInt(f.Enabled), f.DefaultVariant, f.RolloutPercent, string(variantsJSON), string(rulesJSON), string(tagsJSON), string(prereqJSON), f.UpdatedAt, f.Version, f.Key); err != nil {
		return fmt.Errorf("store: update flag: %w", err)
	}
	s.flags[f.Key] = f.Clone()
	s.recordHistory(f)
	return nil
}

// DeleteFlag 删除开关。
func (s *Store) DeleteFlag(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.flags[key]; !ok {
		return fmt.Errorf("store: flag not found: %s", key)
	}
	if _, err := s.db.Exec(`DELETE FROM flags WHERE key=?`, key); err != nil {
		return fmt.Errorf("store: delete flag: %w", err)
	}
	delete(s.flags, key)
	return nil
}

// ---- Segment 读写 ----

// GetSegment 返回分群内存快照。
func (s *Store) GetSegment(id string) (*model.Segment, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seg, ok := s.segments[id]
	if !ok {
		return nil, false
	}
	return seg.Clone(), true
}

// ListSegments 返回全部分群快照。
func (s *Store) ListSegments() []*model.Segment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Segment, 0, len(s.segments))
	for _, seg := range s.segments {
		out = append(out, seg.Clone())
	}
	return out
}

// CreateSegment 创建分群。
func (s *Store) CreateSegment(seg *model.Segment) error {
	if err := seg.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.segments[seg.ID]; exists {
		return fmt.Errorf("store: segment already exists: %s", seg.ID)
	}
	now := model.NowMillis()
	seg.CreatedAt = now
	seg.UpdatedAt = now
	rulesJSON, _ := json.Marshal(seg.Rules)
	membersJSON, _ := json.Marshal(seg.Members)
	if _, err := s.db.Exec(`INSERT INTO segments(id,name,description,rules_json,members_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`,
		seg.ID, seg.Name, seg.Description, string(rulesJSON), string(membersJSON), seg.CreatedAt, seg.UpdatedAt); err != nil {
		return fmt.Errorf("store: insert segment: %w", err)
	}
	s.segments[seg.ID] = seg.Clone()
	return nil
}

// UpdateSegment 更新分群可变字段。
func (s *Store) UpdateSegment(seg *model.Segment) error {
	if err := seg.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.segments[seg.ID]
	if !ok {
		return fmt.Errorf("store: segment not found: %s", seg.ID)
	}
	now := model.NowMillis()
	seg.CreatedAt = cur.CreatedAt
	seg.UpdatedAt = now
	rulesJSON, _ := json.Marshal(seg.Rules)
	membersJSON, _ := json.Marshal(seg.Members)
	if _, err := s.db.Exec(`UPDATE segments SET name=?,description=?,rules_json=?,members_json=?,updated_at=? WHERE id=?`,
		seg.Name, seg.Description, string(rulesJSON), string(membersJSON), seg.UpdatedAt, seg.ID); err != nil {
		return fmt.Errorf("store: update segment: %w", err)
	}
	s.segments[seg.ID] = seg.Clone()
	return nil
}

// DeleteSegment 删除分群。
func (s *Store) DeleteSegment(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.segments[id]; !ok {
		return fmt.Errorf("store: segment not found: %s", id)
	}
	if _, err := s.db.Exec(`DELETE FROM segments WHERE id=?`, id); err != nil {
		return fmt.Errorf("store: delete segment: %w", err)
	}
	delete(s.segments, id)
	return nil
}

// ---- 审计 ----

// AppendAudit 追加一条审计事件。
func (s *Store) AppendAudit(e model.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.ID == "" {
		e.ID = model.AuditID(s.clk.NowMillis(), atomic.AddUint64(&s.evalSeq, 1))
	}
	if e.Ts == 0 {
		e.Ts = s.clk.NowMillis()
	}
	if _, err := s.db.Exec(`INSERT INTO audit(id,ts,action,flag_key,actor,detail) VALUES(?,?,?,?,?,?)`,
		e.ID, e.Ts, e.Action, e.FlagKey, e.Actor, e.Detail); err != nil {
		return fmt.Errorf("store: insert audit: %w", err)
	}
	return nil
}

// ListAudit 返回最近的审计事件（按时间倒序）。
func (s *Store) ListAudit(limit int) ([]model.AuditEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT id,ts,action,flag_key,actor,detail FROM audit ORDER BY ts DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.AuditEvent
	for rows.Next() {
		var e model.AuditEvent
		if err := rows.Scan(&e.ID, &e.Ts, &e.Action, &e.FlagKey, &e.Actor, &e.Detail); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---- 统计与求值计数 ----

// RecordEvaluation 累加求值次数。
func (s *Store) RecordEvaluation() {
	s.evalMu.Lock()
	s.evalCnt++
	s.evalMu.Unlock()
}

// RecordEvaluationResult persists the observable result of a flag evaluation.
// Keeping this history in SQLite makes the report endpoint useful after a
// process restart instead of exposing only an in-memory counter.
func (s *Store) RecordEvaluationResult(flagKey, targetKey string, result model.EvalResult) error {
	s.RecordEvaluation()
	enabled := result.Reason != model.ReasonDisabled && result.Reason != model.ReasonError
	id := fmt.Sprintf("ev_%d_%d", model.NowMillis(), atomic.AddUint64(&s.evalSeq, 1))
	_, err := s.db.Exec(`INSERT INTO evaluations(id,ts,flag_key,target_key,variant_key,enabled,reason) VALUES(?,?,?,?,?,?,?)`,
		id, model.NowMillis(), flagKey, targetKey, result.VariantKey, boolToInt(enabled), result.Reason)
	if err != nil {
		return fmt.Errorf("store: record evaluation: %w", err)
	}
	return nil
}

// ListEvaluationAudits returns persisted evaluation observations for reports.
func (s *Store) ListEvaluationAudits(limit int, flagKey string) ([]evalreport.Audit, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	query := `SELECT ts,flag_key,target_key,variant_key,enabled,reason FROM evaluations ORDER BY ts DESC LIMIT ?`
	args := []any{limit}
	if flagKey != "" {
		query = `SELECT ts,flag_key,target_key,variant_key,enabled,reason FROM evaluations WHERE flag_key=? ORDER BY ts DESC LIMIT ?`
		args = []any{flagKey, limit}
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []evalreport.Audit
	for rows.Next() {
		var item evalreport.Audit
		var enabled int
		if err := rows.Scan(&item.Evaluated, &item.Flag, &item.Identity, &item.Variant, &enabled, &item.Reason); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		out = append(out, item)
	}
	return out, rows.Err()
}

// Stats 返回服务统计。
func (s *Store) Stats() model.Stats {
	s.mu.RLock()
	flagCount := len(s.flags)
	segCount := len(s.segments)
	ruleCount := 0
	for _, f := range s.flags {
		ruleCount += len(f.Rules)
	}
	s.mu.RUnlock()
	var auditCount int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM audit`).Scan(&auditCount)
	s.evalMu.Lock()
	evalCount := s.evalCnt
	s.evalMu.Unlock()
	return model.Stats{
		FlagCount:       flagCount,
		SegmentCount:    segCount,
		RuleCount:       ruleCount,
		AuditCount:      auditCount,
		EvaluationCount: evalCount,
	}
}

// Close 关闭数据库。
func (s *Store) Close() error {
	return s.db.Close()
}

// recordHistory 把开关的当前配置快照写入历史表（每次创建/更新各留一条）。
func (s *Store) recordHistory(f *model.Flag) {
	cfg, err := json.Marshal(f)
	if err != nil {
		return
	}
	id := fmt.Sprintf("fh_%s_%d", f.Key, f.Version)
	_, _ = s.db.Exec(`INSERT OR REPLACE INTO flag_history(id,flag_key,version,config_json,ts) VALUES(?,?,?,?,?)`,
		id, f.Key, f.Version, string(cfg), f.UpdatedAt)
}

// GetFlagHistory 返回某开关的全部历史配置版本（按版本降序）。
func (s *Store) GetFlagHistory(key string) ([]*model.Flag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT config_json FROM flag_history WHERE flag_key=? ORDER BY version DESC`, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Flag
	for rows.Next() {
		var cfg string
		if err := rows.Scan(&cfg); err != nil {
			return nil, err
		}
		var f model.Flag
		if err := json.Unmarshal([]byte(cfg), &f); err != nil {
			return nil, fmt.Errorf("store: unmarshal history: %w", err)
		}
		out = append(out, &f)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
