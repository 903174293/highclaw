package agent

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/highclaw/highclaw/internal/config"
)

func TestArchiveDailyMemoryFiles(t *testing.T) {
	ws := t.TempDir()
	memDir := filepath.Join(ws, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("mkdir memory: %v", err)
	}
	old := time.Now().AddDate(0, 0, -10).Format("2006-01-02") + ".md"
	now := time.Now().Format("2006-01-02") + ".md"
	if err := os.WriteFile(filepath.Join(memDir, old), []byte("old"), 0o644); err != nil {
		t.Fatalf("write old: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memDir, now), []byte("now"), 0o644); err != nil {
		t.Fatalf("write now: %v", err)
	}
	moved := archiveDailyMemoryFiles(ws, 7)
	if moved != 1 {
		t.Fatalf("expected moved=1, got %d", moved)
	}
	if _, err := os.Stat(filepath.Join(memDir, "archive", old)); err != nil {
		t.Fatalf("expected archived file exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(memDir, now)); err != nil {
		t.Fatalf("expected current file kept: %v", err)
	}
}

func TestPruneConversationRows(t *testing.T) {
	ws := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agent.Workspace = ws
	store := newSQLiteMemoryStore(cfg)
	if err := store.init(); err != nil {
		t.Fatalf("init sqlite store: %v", err)
	}

	if err := store.store("conv_old", "old convo", "conversation", memoryMeta{}); err != nil {
		t.Fatalf("store conv old: %v", err)
	}
	if err := store.store("conv_new", "new convo", "conversation", memoryMeta{}); err != nil {
		t.Fatalf("store conv new: %v", err)
	}
	if err := store.store("core_old", "old core", "core", memoryMeta{}); err != nil {
		t.Fatalf("store core old: %v", err)
	}
	oldTs := time.Now().AddDate(0, 0, -40).UTC().Format(time.RFC3339Nano)
	db := store.dbForHygiene()
	if _, err := db.Exec("UPDATE memory_entries SET updated_at = ? WHERE key IN ('conv_old','core_old')", oldTs); err != nil {
		t.Fatalf("set old timestamps: %v", err)
	}

	pruned := pruneConversationRows(ws, 30)
	if pruned != 1 {
		t.Fatalf("expected pruned=1, got %d", pruned)
	}
	rows, err := db.Query("SELECT key FROM memory_entries ORDER BY key")
	if err != nil {
		t.Fatalf("query keys: %v", err)
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		rows.Scan(&k)
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		t.Fatalf("expected remaining rows")
	}
	keySet := strings.Join(keys, ",")
	if strings.Contains(keySet, "conv_old") {
		t.Fatalf("conv_old should be pruned")
	}
	if !strings.Contains(keySet, "conv_new") || !strings.Contains(keySet, "core_old") {
		t.Fatalf("expected conv_new and core_old to remain, got %v", keys)
	}
}

func TestRunMemoryHygieneIfDueRespectsCadence(t *testing.T) {
	ws := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agent.Workspace = ws
	cfg.Memory.HygieneEnabled = true
	cfg.Memory.ArchiveAfterDays = 1
	cfg.Memory.PurgeAfterDays = 1
	cfg.Memory.ConversationRetentionDays = 1

	memDir := filepath.Join(ws, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("mkdir memory: %v", err)
	}
	old := time.Now().AddDate(0, 0, -3).Format("2006-01-02") + ".md"
	if err := os.WriteFile(filepath.Join(memDir, old), []byte("old"), 0o644); err != nil {
		t.Fatalf("write old memory: %v", err)
	}

	statePath := filepath.Join(ws, "state", memoryHygieneStateFile)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}
	state := memoryHygieneState{
		LastRunAt:  time.Now().UTC().Format(time.RFC3339),
		LastReport: memoryHygieneReport{},
	}
	raw, _ := json.Marshal(state)
	if err := os.WriteFile(statePath, raw, 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	runMemoryHygieneIfDue(cfg, logger)

	if _, err := os.Stat(filepath.Join(memDir, old)); err != nil {
		t.Fatalf("old file should remain when cadence not due: %v", err)
	}
	if _, err := os.Stat(filepath.Join(memDir, "archive", old)); err == nil {
		t.Fatalf("file should not be archived when cadence not due")
	}
}

func containsLine(block, target string) bool {
	for _, line := range strings.Split(block, "\n") {
		if strings.TrimSpace(line) == target {
			return true
		}
	}
	return false
}
