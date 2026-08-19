package store

import (
	"database/sql"
	"fmt"
)

// loadEvaluationCount 恢复进程重启前已累计的求值次数。
// 每次求值（RecordEvaluationResult）都会向 evaluations 表写入一行，
// 因此该表的行数即为累计求值次数；重启后从该表恢复，使计数继续累加而非归零。
func loadEvaluationCount(db *sql.DB, count *int64) error {
	var c int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM evaluations`).Scan(&c); err != nil {
		return fmt.Errorf("store: load evaluation count: %w", err)
	}
	*count = c
	return nil
}

// createSchema 在 SQLite 中创建所有持久化表（幂等）。
func createSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS flags (
			key TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 0,
			default_variant TEXT NOT NULL,
			rollout_percent INTEGER NOT NULL DEFAULT 0,
			variants_json TEXT NOT NULL DEFAULT '[]',
			rules_json TEXT NOT NULL DEFAULT '[]',
			tags_json TEXT NOT NULL DEFAULT '[]',
			prerequisites_json TEXT NOT NULL DEFAULT '[]',
			created_at INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL DEFAULT 0,
			version INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS segments (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			rules_json TEXT NOT NULL DEFAULT '[]',
			members_json TEXT NOT NULL DEFAULT '[]',
			created_at INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS audit (
			id TEXT PRIMARY KEY,
			ts INTEGER NOT NULL,
			action TEXT NOT NULL,
			flag_key TEXT NOT NULL DEFAULT '',
			actor TEXT NOT NULL DEFAULT '',
			detail TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_ts ON audit(ts)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_flag ON audit(flag_key)`,
		`CREATE TABLE IF NOT EXISTS flag_history (
			id TEXT PRIMARY KEY,
			flag_key TEXT NOT NULL,
			version INTEGER NOT NULL,
			config_json TEXT NOT NULL,
			ts INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_flag_history_key ON flag_history(flag_key, version)`,
		`CREATE TABLE IF NOT EXISTS evaluations (
			id TEXT PRIMARY KEY,
			ts INTEGER NOT NULL,
			flag_key TEXT NOT NULL,
			target_key TEXT NOT NULL DEFAULT '',
			variant_key TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 0,
			reason TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_evaluations_flag_ts ON evaluations(flag_key, ts)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
