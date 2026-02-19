// Package logger 提供文件级日志管理，支持日志轮转、多级别输出和 stderr 双写。
// 日志文件存储在 ~/.highclaw/logs/ 目录，按日期自动轮转，
// 确保服务无法启动时也可通过原始日志文件排查问题。
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Config 日志管理器配置
type Config struct {
	Dir           string     `json:"dir"`           // 日志目录，默认 ~/.highclaw/logs
	Level         slog.Level `json:"level"`         // 最低日志级别
	MaxAgeDays    int        `json:"maxAgeDays"`    // 日志保留天数，0 不清理
	MaxSizeMB     int        `json:"maxSizeMB"`     // 单文件最大 MB，超过轮转
	StderrEnabled bool       `json:"stderrEnabled"` // 是否双写到 stderr
}

// Manager 管理日志文件生命周期
type Manager struct {
	cfg     Config
	mu      sync.Mutex
	file    *os.File
	curDate string
}

// DefaultConfig 返回默认日志配置
func DefaultConfig() Config {
	return Config{
		Dir:           defaultLogDir(),
		Level:         slog.LevelInfo,
		MaxAgeDays:    30,
		MaxSizeMB:     50,
		StderrEnabled: true,
	}
}

func defaultLogDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".highclaw", "logs")
	}
	return filepath.Join(home, ".highclaw", "logs")
}

// New 创建日志管理器并初始化日志文件
func New(cfg Config) (*Manager, error) {
	if cfg.Dir == "" {
		cfg.Dir = defaultLogDir()
	}
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = 50
	}
	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	m := &Manager{cfg: cfg}
	if err := m.rotateIfNeeded(); err != nil {
		return nil, err
	}
	return m, nil
}

// NewSlogHandler 创建写入日志文件的 slog.Handler
func (m *Manager) NewSlogHandler() slog.Handler {
	return slog.NewTextHandler(m, &slog.HandlerOptions{
		Level: m.cfg.Level,
	})
}

// NewLogger 返回基于文件的 slog.Logger
func (m *Manager) NewLogger() *slog.Logger {
	return slog.New(m.NewSlogHandler())
}

// Write 实现 io.Writer，按日期轮转，可选 stderr 双写
func (m *Manager) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_ = m.rotateIfNeededLocked()

	if m.file != nil {
		n, err = m.file.Write(p)
	}

	if m.cfg.StderrEnabled {
		_, _ = os.Stderr.Write(p)
	}

	return n, err
}

// Close 关闭日志文件
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.file != nil {
		err := m.file.Close()
		m.file = nil
		return err
	}
	return nil
}

// LogDir 返回日志目录路径
func (m *Manager) LogDir() string {
	return m.cfg.Dir
}

// CurrentLogFile 返回当前日志文件路径
func (m *Manager) CurrentLogFile() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.file != nil {
		return m.file.Name()
	}
	return logFileName(m.cfg.Dir, todayDate())
}

func (m *Manager) rotateIfNeeded() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rotateIfNeededLocked()
}

func (m *Manager) rotateIfNeededLocked() error {
	today := todayDate()
	needRotate := false

	if m.file == nil {
		needRotate = true
	} else if m.curDate != today {
		needRotate = true
	} else if m.cfg.MaxSizeMB > 0 {
		if info, err := m.file.Stat(); err == nil {
			if info.Size() >= int64(m.cfg.MaxSizeMB)*1024*1024 {
				needRotate = true
			}
		}
	}

	if !needRotate {
		return nil
	}

	if m.file != nil {
		_ = m.file.Close()
		m.file = nil
	}

	path := logFileName(m.cfg.Dir, today)
	if m.cfg.MaxSizeMB > 0 {
		if info, err := os.Stat(path); err == nil && info.Size() >= int64(m.cfg.MaxSizeMB)*1024*1024 {
			for seq := 1; seq < 100; seq++ {
				candidate := filepath.Join(m.cfg.Dir, fmt.Sprintf("highclaw-%s.%d.log", today, seq))
				if _, err := os.Stat(candidate); os.IsNotExist(err) {
					path = candidate
					break
				}
			}
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	m.file = f
	m.curDate = today
	return nil
}

// Cleanup 清理过期日志文件
func (m *Manager) Cleanup() (int, error) {
	if m.cfg.MaxAgeDays <= 0 {
		return 0, nil
	}
	entries, err := os.ReadDir(m.cfg.Dir)
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().AddDate(0, 0, -m.cfg.MaxAgeDays)
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(m.cfg.Dir, entry.Name())); err == nil {
				removed++
			}
		}
	}
	return removed, nil
}

// ListLogFiles 列出所有日志文件，按时间倒序
func ListLogFiles(dir string) ([]LogFileInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []LogFileInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, LogFileInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(dir, entry.Name()),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})
	return files, nil
}

// LogFileInfo 描述单个日志文件
type LogFileInfo struct {
	Name    string
	Path    string
	Size    int64
	ModTime time.Time
}

// TotalSize 返回日志目录总大小（字节）
func TotalSize(dir string) (int64, error) {
	files, err := ListLogFiles(dir)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, f := range files {
		total += f.Size
	}
	return total, nil
}

// TailFile 读取日志文件最后 n 行
func TailFile(path string, n int) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	if n <= 0 {
		n = 200
	}
	start := len(lines) - n
	if start < 0 {
		start = 0
	}
	var result []string
	for i := start; i < len(lines); i++ {
		if lines[i] != "" {
			result = append(result, lines[i])
		}
	}
	return result, nil
}

// QueryFile 在日志文件中搜索匹配行
func QueryFile(path, pattern string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(pattern)
	var matches []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(strings.ToLower(line), q) {
			matches = append(matches, line)
		}
	}
	return matches, nil
}

// FollowFile 追踪日志文件新内容直到 stop 通道关闭
func FollowFile(path string, w io.Writer, stop <-chan struct{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	buf := make([]byte, 4096)
	for {
		select {
		case <-stop:
			return nil
		default:
		}
		n, readErr := f.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
		}
		if readErr != nil {
			if readErr == io.EOF {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return readErr
		}
	}
}

func todayDate() string {
	return time.Now().Format("2006-01-02")
}

func logFileName(dir, date string) string {
	return filepath.Join(dir, fmt.Sprintf("highclaw-%s.log", date))
}
