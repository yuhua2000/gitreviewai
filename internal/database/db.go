package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	if err := InitBuiltinRules(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("init builtin rules: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS merge_requests (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id    TEXT    NOT NULL,
			mr_iid        INTEGER NOT NULL,
			title         TEXT    NOT NULL DEFAULT '',
			description   TEXT    NOT NULL DEFAULT '',
			source_branch TEXT    NOT NULL DEFAULT '',
			target_branch TEXT    NOT NULL DEFAULT '',
			state         TEXT    NOT NULL DEFAULT 'open',
			web_url       TEXT    NOT NULL DEFAULT '',
			review_status TEXT    NOT NULL DEFAULT 'pending',
			created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at    DATETIME NOT NULL DEFAULT (datetime('now')),
			UNIQUE(project_id, mr_iid)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mr_project_iid ON merge_requests(project_id, mr_iid)`,
		`CREATE INDEX IF NOT EXISTS idx_mr_review_status ON merge_requests(review_status)`,
		`CREATE INDEX IF NOT EXISTS idx_mr_created_at ON merge_requests(created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS comments (
			id                   INTEGER PRIMARY KEY AUTOINCREMENT,
			mr_id                INTEGER NOT NULL,
			comment_type         TEXT    NOT NULL,
			file_path            TEXT    NOT NULL DEFAULT '',
			line_number          INTEGER NOT NULL DEFAULT 0,
			content              TEXT    NOT NULL DEFAULT '',
			diff_context         TEXT    NOT NULL DEFAULT '',
			status               TEXT    NOT NULL DEFAULT 'pending',
			gitlab_note_id       INTEGER,
			gitlab_draft_note_id INTEGER,
			submitted_at         DATETIME,
			created_at           DATETIME NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (mr_id) REFERENCES merge_requests(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_comment_mr_id ON comments(mr_id)`,
		`CREATE INDEX IF NOT EXISTS idx_comment_status ON comments(status)`,
		`CREATE TABLE IF NOT EXISTS reports (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			mr_id          INTEGER NOT NULL,
			content        TEXT    NOT NULL DEFAULT '',
			status         TEXT    NOT NULL DEFAULT 'pending',
			gitlab_note_id INTEGER,
			submitted_at   DATETIME,
			created_at     DATETIME NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (mr_id) REFERENCES merge_requests(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_report_mr_id ON reports(mr_id)`,
		`CREATE INDEX IF NOT EXISTS idx_report_status ON reports(status)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key       TEXT PRIMARY KEY,
			value     TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS ai_models (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT NOT NULL UNIQUE,
			base_url   TEXT NOT NULL DEFAULT '',
			api_key    TEXT NOT NULL DEFAULT '',
			model_name TEXT NOT NULL DEFAULT '',
			is_default BOOLEAN NOT NULL DEFAULT 0,
			enabled    BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS review_rules (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			rule_id     TEXT NOT NULL UNIQUE,
			name        TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			severity    TEXT NOT NULL DEFAULT 'warning',
			is_builtin  BOOLEAN NOT NULL DEFAULT 1,
			enabled     BOOLEAN NOT NULL DEFAULT 1,
			created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS project_configs (
			id                INTEGER PRIMARY KEY AUTOINCREMENT,
			gitlab_project_id INTEGER NOT NULL UNIQUE,
			project_name      TEXT NOT NULL DEFAULT '',
			description       TEXT NOT NULL DEFAULT '',
			ai_model_id       INTEGER,
			auto_submit       BOOLEAN NOT NULL DEFAULT 0,
			skip_draft        BOOLEAN NOT NULL DEFAULT 1,
			target_branches   TEXT NOT NULL DEFAULT '[]',
			ignore_paths      TEXT NOT NULL DEFAULT '[]',
			max_line_comments INTEGER,
			custom_prompt     TEXT NOT NULL DEFAULT '',
			enabled           BOOLEAN NOT NULL DEFAULT 1,
			created_at        DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at        DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_project_configs_gitlab_id ON project_configs(gitlab_project_id)`,
		`CREATE TABLE IF NOT EXISTS project_rule_overrides (
			id                INTEGER PRIMARY KEY AUTOINCREMENT,
			project_config_id INTEGER NOT NULL,
			rule_id           TEXT NOT NULL,
			enabled           BOOLEAN NOT NULL DEFAULT 1,
			UNIQUE(project_config_id, rule_id),
			FOREIGN KEY (project_config_id) REFERENCES project_configs(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_project_rule_overrides_config_id ON project_rule_overrides(project_config_id)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("exec migration: %w\nQuery: %s", err, q)
		}
	}

	return nil
}

// InitBuiltinRules inserts the predefined review rules if they don't already exist.
func InitBuiltinRules(db *sql.DB) error {
	rules := []struct {
		RuleID      string
		Name        string
		Description string
		Severity    string
		Enabled     bool
	}{
		// ERROR level
		{"logic-error", "逻辑错误", "条件判断错误、循环边界、空值处理", "error", true},
		{"security", "安全漏洞", "SQL 注入、XSS、敏感信息泄露、权限绕过", "error", true},
		{"concurrency", "并发问题", "竞态条件、死锁、资源未释放", "error", true},
		{"error-handling", "错误处理", "异常吞没、错误未传递、资源泄漏", "error", true},
		{"data-consistency", "数据一致性", "缺少事务、状态不同步", "error", true},
		// WARNING level
		{"performance", "性能问题", "不必要的循环、内存分配、N+1 查询", "warning", true},
		{"code-standards", "代码规范", "命名不清、魔法数字、重复代码", "warning", true},
		{"maintainability", "可维护性", "函数过长、嵌套过深、缺少注释", "warning", true},
		{"code-format", "代码格式", "缩进不一致、尾随空格、缺少空行、import 排序", "warning", true},
		// INFO level
		{"best-practice", "最佳实践", "语言惯用写法、社区最佳实践、设计模式建议", "info", false},
		{"documentation", "文档建议", "缺少函数/模块注释、API 文档不完整", "info", false},
	}

	stmt, err := db.Prepare(`INSERT OR IGNORE INTO review_rules (rule_id, name, description, severity, is_builtin, enabled) VALUES (?, ?, ?, ?, 1, ?)`)
	if err != nil {
		return fmt.Errorf("prepare builtin rules insert: %w", err)
	}
	defer stmt.Close()

	for _, r := range rules {
		if _, err := stmt.Exec(r.RuleID, r.Name, r.Description, r.Severity, r.Enabled); err != nil {
			return fmt.Errorf("insert builtin rule %s: %w", r.RuleID, err)
		}
	}

	return nil
}
