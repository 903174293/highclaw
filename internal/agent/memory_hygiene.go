package agent

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/highclaw/highclaw/internal/config"
	_ "modernc.org/sqlite"
)

const (
	memoryHygieneStateFile = "memory_hygiene_state.json"
	memoryHygieneInterval  = 12 * time.Hour
)

type memoryHygieneReport struct {
	ArchivedMemoryFiles    uint64 `json:"archived_memory_files"`
	ArchivedSessionFiles   uint64 `json:"archived_session_files"`
	PurgedMemoryArchives   uint64 `json:"purged_memory_archives"`
	PurgedSessionArchives  uint64 `json:"purged_session_archives"`
	PrunedConversationRows uint64 `json:"pruned_conversation_rows"`
}

func (r memoryHygieneReport) totalActions() uint64 {
	return r.ArchivedMemoryFiles + r.ArchivedSessionFiles + r.PurgedMemoryArchives + r.PurgedSessionArchives + r.PrunedConversationRows
}

type memoryHygieneState struct {
	LastRunAt  string              `json:"last_run_at"`
	LastReport memoryHygieneReport `json:"last_report"`
}

func runMemoryHygieneIfDue(cfg *config.Config, logger *slog.Logger) {
	if cfg == nil || !cfg.Memory.HygieneEnabled {
		return
	}
	workspace := strings.TrimSpace(cfg.Agent.Workspace)
	if workspace == "" {
		return
	}
	statePath := filepath.Join(workspace, "state", memoryHygieneStateFile)
	if !shouldRunMemoryHygiene(statePath) {
		return
	}

	report := memoryHygieneReport{}
	report.ArchivedMemoryFiles = archiveDailyMemoryFiles(workspace, cfg.Memory.ArchiveAfterDays)
	report.ArchivedSessionFiles = archiveSessionFiles(workspace, cfg.Memory.ArchiveAfterDays)
	report.PurgedMemoryArchives = purgeMemoryArchives(workspace, cfg.Memory.PurgeAfterDays)
	report.PurgedSessionArchives = purgeSessionArchives(workspace, cfg.Memory.PurgeAfterDays)
	report.PrunedConversationRows = pruneConversationRows(workspace, cfg.Memory.ConversationRetentionDays)

	writeMemoryHygieneState(statePath, report)
	if report.totalActions() > 0 {
		logger.Info("memory hygiene complete",
			"archived_memory", report.ArchivedMemoryFiles,
			"archived_sessions", report.ArchivedSessionFiles,
			"purged_memory", report.PurgedMemoryArchives,
			"purged_sessions", report.PurgedSessionArchives,
			"pruned_conversation_rows", report.PrunedConversationRows,
		)
	}
}

func shouldRunMemoryHygiene(statePath string) bool {
	raw, err := os.ReadFile(statePath)
	if err != nil {
		return true
	}
	var st memoryHygieneState
	if err := json.Unmarshal(raw, &st); err != nil {
		return true
	}
	if strings.TrimSpace(st.LastRunAt) == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339, st.LastRunAt)
	if err != nil {
		return true
	}
	return time.Since(last) >= memoryHygieneInterval
}

func writeMemoryHygieneState(statePath string, report memoryHygieneReport) {
	_ = os.MkdirAll(filepath.Dir(statePath), 0o755)
	st := memoryHygieneState{LastRunAt: time.Now().UTC().Format(time.RFC3339), LastReport: report}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(statePath, raw, 0o644)
}

func archiveDailyMemoryFiles(workspace string, days int) uint64 {
	if days <= 0 {
		return 0
	}
	memoryDir := filepath.Join(workspace, "memory")
	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		return 0
	}
	archiveDir := filepath.Join(memoryDir, "archive")
	_ = os.MkdirAll(archiveDir, 0o755)
	cutoff := time.Now().AddDate(0, 0, -days)
	moved := uint64(0)
	for _, de := range entries {
		if de.IsDir() || filepath.Ext(de.Name()) != ".md" {
			continue
		}
		datePart := strings.TrimSuffix(de.Name(), ".md")
		d, err := time.Parse("2006-01-02", datePart)
		if err != nil {
			continue
		}
		if d.Before(cutoff) {
			src := filepath.Join(memoryDir, de.Name())
			dst := uniqueArchiveTarget(archiveDir, de.Name())
			if err := os.Rename(src, dst); err == nil {
				moved++
			}
		}
	}
	return moved
}

func archiveSessionFiles(workspace string, days int) uint64 {
	if days <= 0 {
		return 0
	}
	sessionsDir := filepath.Join(workspace, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return 0
	}
	archiveDir := filepath.Join(sessionsDir, "archive")
	_ = os.MkdirAll(archiveDir, 0o755)
	cutoff := time.Now().AddDate(0, 0, -days)
	moved := uint64(0)
	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		src := filepath.Join(sessionsDir, de.Name())
		isOld := false
		if d, ok := datePrefixFromFilename(de.Name()); ok {
			isOld = d.Before(cutoff)
		} else if info, err := os.Stat(src); err == nil {
			isOld = info.ModTime().Before(cutoff)
		}
		if isOld {
			dst := uniqueArchiveTarget(archiveDir, de.Name())
			if err := os.Rename(src, dst); err == nil {
				moved++
			}
		}
	}
	return moved
}

func purgeMemoryArchives(workspace string, days int) uint64 {
	if days <= 0 {
		return 0
	}
	archiveDir := filepath.Join(workspace, "memory", "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		return 0
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	removed := uint64(0)
	for _, de := range entries {
		if de.IsDir() || filepath.Ext(de.Name()) != ".md" {
			continue
		}
		datePart := strings.TrimSuffix(de.Name(), ".md")
		datePart = strings.Split(datePart, "_")[0]
		d, err := time.Parse("2006-01-02", datePart)
		if err != nil {
			continue
		}
		if d.Before(cutoff) {
			if err := os.Remove(filepath.Join(archiveDir, de.Name())); err == nil {
				removed++
			}
		}
	}
	return removed
}

func purgeSessionArchives(workspace string, days int) uint64 {
	if days <= 0 {
		return 0
	}
	archiveDir := filepath.Join(workspace, "sessions", "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		return 0
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	removed := uint64(0)
	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		p := filepath.Join(archiveDir, de.Name())
		isOld := false
		if d, ok := datePrefixFromFilename(de.Name()); ok {
			isOld = d.Before(cutoff)
		} else if info, err := os.Stat(p); err == nil {
			isOld = info.ModTime().Before(cutoff)
		}
		if isOld {
			if err := os.Remove(p); err == nil {
				removed++
			}
		}
	}
	return removed
}

// pruneConversationRows 使用 in-process SQLite 清理过期对话记忆
func pruneConversationRows(workspace string, days int) uint64 {
	if days <= 0 {
		return 0
	}
	dbPath := filepath.Join(workspace, "memory", "brain.db")
	if _, err := os.Stat(dbPath); err != nil {
		return 0
	}
	cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339Nano)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return 0
	}
	defer db.Close()
	result, err := db.Exec("DELETE FROM memory_entries WHERE category='conversation' AND updated_at < ?", cutoff)
	if err != nil {
		return 0
	}
	affected, err := result.RowsAffected()
	if err != nil || affected < 0 {
		return 0
	}
	return uint64(affected)
}

func uniqueArchiveTarget(dir, filename string) string {
	direct := filepath.Join(dir, filename)
	if _, err := os.Stat(direct); os.IsNotExist(err) {
		return direct
	}
	ext := filepath.Ext(filename)
	stem := strings.TrimSuffix(filename, ext)
	for i := 1; i < 10000; i++ {
		candidate := fmt.Sprintf("%s_%d%s", stem, i, ext)
		path := filepath.Join(dir, candidate)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path
		}
	}
	return direct
}

func datePrefixFromFilename(filename string) (time.Time, bool) {
	if len(filename) < 10 {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", filename[:10])
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
