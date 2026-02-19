// Package tasklog 提供基于 SQLite 的任务审计日志。
// 记录用户所有操作（增删查改、聊天等），包括请求/响应详情、时间、会话信息。
// 存储位置: ~/.highclaw/state/tasks.db，与业务数据完全解耦。
package tasklog

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// ActionType 操作类型枚举
const (
	ActionChat    = "chat"    // 聊天消息
	ActionCreate  = "create"  // 新建操作
	ActionRead    = "read"    // 查询操作
	ActionUpdate  = "update"  // 更新操作
	ActionDelete  = "delete"  // 删除操作
	ActionTool    = "tool"    // 工具执行
	ActionSystem  = "system"  // 系统操作（启动、停止等）
	ActionConfig  = "config"  // 配置变更
	ActionMemory  = "memory"  // 记忆操作
	ActionChannel = "channel" // 渠道操作
)

// Config 任务日志配置
type Config struct {
	Dir        string `json:"dir"`        // 数据库目录，默认 ~/.highclaw/state
	MaxAgeDays int    `json:"maxAgeDays"` // 记录保留天数，0 不清理
	MaxRecords int    `json:"maxRecords"` // 最大记录数，0 不限制
	Enabled    bool   `json:"enabled"`    // 是否启用
}

// TaskRecord 单条任务记录
type TaskRecord struct {
	ID           int64  `json:"id"`
	Action       string `json:"action"`       // 操作类型
	Module       string `json:"module"`       // 所属模块
	SessionKey   string `json:"sessionKey"`   // 会话标识
	Channel      string `json:"channel"`      // 渠道
	Sender       string `json:"sender"`       // 发送者
	RequestBody  string `json:"requestBody"`  // 请求详情
	ResponseBody string `json:"responseBody"` // 响应详情
	Status       string `json:"status"`       // success / error
	ErrorMessage string `json:"errorMessage"` // 错误信息
	DurationMs   int64  `json:"durationMs"`   // 执行耗时（毫秒）
	TokensInput  int    `json:"tokensInput"`  // 输入 token 数
	TokensOutput int    `json:"tokensOutput"` // 输出 token 数
	Model        string `json:"model"`        // 使用的模型
	CreatedAt    string `json:"createdAt"`    // 创建时间
}

// Store 任务日志存储引擎
type Store struct {
	dbPath string
	db     *sql.DB
	mu     sync.Mutex
}

// DefaultConfig 返回默认任务日志配置
func DefaultConfig() Config {
	return Config{
		Dir:        defaultStateDir(),
		MaxAgeDays: 90,
		MaxRecords: 100000,
		Enabled:    true,
	}
}

func defaultStateDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".highclaw", "state")
	}
	return filepath.Join(home, ".highclaw", "state")
}

// NewStore 创建任务日志存储
func NewStore(cfg Config) (*Store, error) {
	if cfg.Dir == "" {
		cfg.Dir = defaultStateDir()
	}
	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("create tasklog dir: %w", err)
	}
	dbPath := filepath.Join(cfg.Dir, "tasks.db")
	s := &Store{dbPath: dbPath}
	if err := s.init(); err != nil {
		return nil, err
	}
	return s, nil
}

// init 初始化数据库表结构和索引
func (s *Store) init() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	db, err := s.openDB()
	if err != nil {
		return err
	}

	ddl := `
CREATE TABLE IF NOT EXISTS task_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  action TEXT NOT NULL DEFAULT '',
  module TEXT NOT NULL DEFAULT '',
  session_key TEXT NOT NULL DEFAULT '',
  channel TEXT NOT NULL DEFAULT '',
  sender TEXT NOT NULL DEFAULT '',
  request_body TEXT NOT NULL DEFAULT '',
  response_body TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'success',
  error_message TEXT NOT NULL DEFAULT '',
  duration_ms INTEGER NOT NULL DEFAULT 0,
  tokens_input INTEGER NOT NULL DEFAULT 0,
  tokens_output INTEGER NOT NULL DEFAULT 0,
  model TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);`
	if _, err := db.Exec(ddl); err != nil {
		return fmt.Errorf("create task_records table: %w", err)
	}

	indices := []string{
		"CREATE INDEX IF NOT EXISTS idx_task_records_created ON task_records(created_at DESC);",
		"CREATE INDEX IF NOT EXISTS idx_task_records_action ON task_records(action);",
		"CREATE INDEX IF NOT EXISTS idx_task_records_module ON task_records(module);",
		"CREATE INDEX IF NOT EXISTS idx_task_records_session ON task_records(session_key);",
		"CREATE INDEX IF NOT EXISTS idx_task_records_channel ON task_records(channel);",
		"CREATE INDEX IF NOT EXISTS idx_task_records_status ON task_records(status);",
	}
	for _, idx := range indices {
		_, _ = db.Exec(idx)
	}

	// FTS5 全文搜索索引
	_, _ = db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS task_records_fts USING fts5(
		request_body, response_body, error_message,
		content=task_records, content_rowid=id
	);`)
	_, _ = db.Exec(`CREATE TRIGGER IF NOT EXISTS task_records_fts_ai AFTER INSERT ON task_records BEGIN
		INSERT INTO task_records_fts(rowid, request_body, response_body, error_message) VALUES (new.id, new.request_body, new.response_body, new.error_message);
	END;`)
	_, _ = db.Exec(`CREATE TRIGGER IF NOT EXISTS task_records_fts_ad AFTER DELETE ON task_records BEGIN
		INSERT INTO task_records_fts(task_records_fts, rowid, request_body, response_body, error_message) VALUES ('delete', old.id, old.request_body, old.response_body, old.error_message);
	END;`)

	return nil
}

func (s *Store) openDB() (*sql.DB, error) {
	if s.db != nil {
		return s.db, nil
	}
	db, err := sql.Open("sqlite", s.dbPath+"?_pragma=busy_timeout%3d5000&_pragma=journal_mode%3dwal")
	if err != nil {
		return nil, fmt.Errorf("open tasklog db: %w", err)
	}
	db.SetMaxOpenConns(1)
	s.db = db
	return db, nil
}

// Log 记录一条任务
func (s *Store) Log(rec *TaskRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	db, err := s.openDB()
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if rec.CreatedAt == "" {
		rec.CreatedAt = now
	}
	if rec.Status == "" {
		rec.Status = "success"
	}

	result, err := db.Exec(
		`INSERT INTO task_records(action, module, session_key, channel, sender, request_body, response_body, status, error_message, duration_ms, tokens_input, tokens_output, model, created_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		rec.Action, rec.Module, rec.SessionKey, rec.Channel, rec.Sender,
		rec.RequestBody, rec.ResponseBody, rec.Status, rec.ErrorMessage,
		rec.DurationMs, rec.TokensInput, rec.TokensOutput, rec.Model, rec.CreatedAt,
	)
	if err != nil {
		return err
	}
	rec.ID, _ = result.LastInsertId()
	return nil
}

// Get 根据 ID 获取单条记录
func (s *Store) Get(id int64) (*TaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	db, err := s.openDB()
	if err != nil {
		return nil, err
	}

	row := db.QueryRow(
		`SELECT id, action, module, session_key, channel, sender, request_body, response_body, status, error_message, duration_ms, tokens_input, tokens_output, model, created_at
		 FROM task_records WHERE id=?`, id,
	)
	return scanRecord(row)
}

// QueryParams 查询参数
type QueryParams struct {
	Action     string // 按操作类型过滤
	Module     string // 按模块过滤
	SessionKey string // 按会话过滤
	Channel    string // 按渠道过滤
	Status     string // 按状态过滤
	Search     string // 全文搜索
	Since      string // 起始时间（RFC3339），含
	Until      string // 截止时间（RFC3339），含
	SortBy     string // 排序字段: created_at(默认) | duration_ms | tokens_input | tokens_output | action | module
	SortDesc   bool   // 是否降序，默认 true
	Limit      int    // 分页大小
	Offset     int    // 偏移量
}

// Query 分页查询任务记录
func (s *Store) Query(p QueryParams) ([]TaskRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	db, err := s.openDB()
	if err != nil {
		return nil, 0, err
	}

	if p.Limit <= 0 {
		p.Limit = 50
	}

	var conditions []string
	var args []any

	if p.Action != "" {
		conditions = append(conditions, "action=?")
		args = append(args, p.Action)
	}
	if p.Module != "" {
		conditions = append(conditions, "module=?")
		args = append(args, p.Module)
	}
	if p.SessionKey != "" {
		conditions = append(conditions, "session_key=?")
		args = append(args, p.SessionKey)
	}
	if p.Channel != "" {
		conditions = append(conditions, "channel=?")
		args = append(args, p.Channel)
	}
	if p.Status != "" {
		conditions = append(conditions, "status=?")
		args = append(args, p.Status)
	}

	// 全文搜索走 FTS5
	if p.Search != "" {
		ftsQuery := buildFTSQuery(p.Search)
		conditions = append(conditions, "id IN (SELECT rowid FROM task_records_fts WHERE task_records_fts MATCH ?)")
		args = append(args, ftsQuery)
	}
	if p.Since != "" {
		conditions = append(conditions, "created_at>=?")
		args = append(args, p.Since)
	}
	if p.Until != "" {
		conditions = append(conditions, "created_at<=?")
		args = append(args, p.Until)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	// 统计总数
	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	_ = db.QueryRow("SELECT COUNT(*) FROM task_records"+where, countArgs...).Scan(&total)

	// 排序
	sortCol := "created_at"
	allowedSortCols := map[string]bool{
		"created_at": true, "duration_ms": true,
		"tokens_input": true, "tokens_output": true,
		"action": true, "module": true, "status": true,
	}
	if p.SortBy != "" && allowedSortCols[p.SortBy] {
		sortCol = p.SortBy
	}
	sortDir := "DESC"
	if !p.SortDesc && p.SortBy != "" {
		sortDir = "ASC"
	}

	// 查询数据
	query := "SELECT id, action, module, session_key, channel, sender, request_body, response_body, status, error_message, duration_ms, tokens_input, tokens_output, model, created_at FROM task_records" + where + " ORDER BY " + sortCol + " " + sortDir + " LIMIT ? OFFSET ?"
	args = append(args, p.Limit, p.Offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []TaskRecord
	for rows.Next() {
		var r TaskRecord
		if err := rows.Scan(&r.ID, &r.Action, &r.Module, &r.SessionKey, &r.Channel, &r.Sender,
			&r.RequestBody, &r.ResponseBody, &r.Status, &r.ErrorMessage,
			&r.DurationMs, &r.TokensInput, &r.TokensOutput, &r.Model, &r.CreatedAt); err != nil {
			continue
		}
		records = append(records, r)
	}
	return records, total, rows.Err()
}

// Stats 返回统计信息
type Stats struct {
	TotalRecords   int            `json:"totalRecords"`
	TotalTokensIn  int64          `json:"totalTokensIn"`
	TotalTokensOut int64          `json:"totalTokensOut"`
	ByAction       map[string]int `json:"byAction"`
	ByModule       map[string]int `json:"byModule"`
	ByStatus       map[string]int `json:"byStatus"`
	AvgDurationMs  float64        `json:"avgDurationMs"`
	EarliestRecord string         `json:"earliestRecord"`
	LatestRecord   string         `json:"latestRecord"`
}

// GetStats 获取任务日志统计
func (s *Store) GetStats() (*Stats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	db, err := s.openDB()
	if err != nil {
		return nil, err
	}

	st := &Stats{
		ByAction: make(map[string]int),
		ByModule: make(map[string]int),
		ByStatus: make(map[string]int),
	}

	_ = db.QueryRow("SELECT COUNT(*) FROM task_records").Scan(&st.TotalRecords)
	_ = db.QueryRow("SELECT COALESCE(SUM(tokens_input),0) FROM task_records").Scan(&st.TotalTokensIn)
	_ = db.QueryRow("SELECT COALESCE(SUM(tokens_output),0) FROM task_records").Scan(&st.TotalTokensOut)
	_ = db.QueryRow("SELECT COALESCE(AVG(duration_ms),0) FROM task_records WHERE duration_ms>0").Scan(&st.AvgDurationMs)
	_ = db.QueryRow("SELECT COALESCE(MIN(created_at),'') FROM task_records").Scan(&st.EarliestRecord)
	_ = db.QueryRow("SELECT COALESCE(MAX(created_at),'') FROM task_records").Scan(&st.LatestRecord)

	scanGroupBy(db, "SELECT action, COUNT(*) FROM task_records GROUP BY action", st.ByAction)
	scanGroupBy(db, "SELECT module, COUNT(*) FROM task_records GROUP BY module", st.ByModule)
	scanGroupBy(db, "SELECT status, COUNT(*) FROM task_records GROUP BY status", st.ByStatus)

	return st, nil
}

// Cleanup 清理过期记录
func (s *Store) Cleanup(maxAgeDays, maxRecords int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	db, err := s.openDB()
	if err != nil {
		return 0, err
	}

	var totalDeleted int64

	// 按时间清理
	if maxAgeDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -maxAgeDays).UTC().Format(time.RFC3339Nano)
		result, err := db.Exec("DELETE FROM task_records WHERE created_at < ?", cutoff)
		if err == nil {
			n, _ := result.RowsAffected()
			totalDeleted += n
		}
	}

	// 按数量清理（保留最新的 maxRecords 条）
	if maxRecords > 0 {
		result, err := db.Exec(
			"DELETE FROM task_records WHERE id NOT IN (SELECT id FROM task_records ORDER BY created_at DESC LIMIT ?)",
			maxRecords,
		)
		if err == nil {
			n, _ := result.RowsAffected()
			totalDeleted += n
		}
	}

	// 重建 FTS 索引
	if totalDeleted > 0 {
		_, _ = db.Exec("INSERT INTO task_records_fts(task_records_fts) VALUES('rebuild')")
	}

	return totalDeleted, nil
}

// Count 返回总记录数
func (s *Store) Count() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	db, err := s.openDB()
	if err != nil {
		return 0, err
	}
	var cnt int
	_ = db.QueryRow("SELECT COUNT(*) FROM task_records").Scan(&cnt)
	return cnt, nil
}

// Close 关闭数据库
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		err := s.db.Close()
		s.db = nil
		return err
	}
	return nil
}

// DBPath 返回数据库文件路径
func (s *Store) DBPath() string {
	return s.dbPath
}

func scanRecord(row *sql.Row) (*TaskRecord, error) {
	var r TaskRecord
	err := row.Scan(&r.ID, &r.Action, &r.Module, &r.SessionKey, &r.Channel, &r.Sender,
		&r.RequestBody, &r.ResponseBody, &r.Status, &r.ErrorMessage,
		&r.DurationMs, &r.TokensInput, &r.TokensOutput, &r.Model, &r.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

func scanGroupBy(db *sql.DB, query string, target map[string]int) {
	rows, err := db.Query(query)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var cnt int
		if err := rows.Scan(&key, &cnt); err == nil {
			target[key] = cnt
		}
	}
}

func buildFTSQuery(input string) string {
	words := strings.Fields(strings.TrimSpace(input))
	if len(words) == 0 {
		return `""`
	}
	parts := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		w = strings.ReplaceAll(w, `"`, `""`)
		parts = append(parts, `"`+w+`"`)
	}
	if len(parts) == 0 {
		return `""`
	}
	return strings.Join(parts, " OR ")
}
