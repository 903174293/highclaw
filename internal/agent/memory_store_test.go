package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMarkdownMemoryStoreCorePathAndFormat(t *testing.T) {
	workspace := t.TempDir()
	store := newMarkdownMemoryStore(workspace)
	if err := store.init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := store.store("user_pref", "likes rust", "core", memoryMeta{}); err != nil {
		t.Fatalf("store core: %v", err)
	}

	corePath := filepath.Join(workspace, "MEMORY.md")
	raw, err := os.ReadFile(corePath)
	if err != nil {
		t.Fatalf("read MEMORY.md: %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, "# Long-Term Memory") {
		t.Fatalf("missing core header: %q", content)
	}
	if !strings.Contains(content, "- **user_pref**: likes rust") {
		t.Fatalf("missing core entry: %q", content)
	}
}

func TestMarkdownMemoryStoreDailyPathAndRecall(t *testing.T) {
	workspace := t.TempDir()
	store := newMarkdownMemoryStore(workspace)
	if err := store.init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := store.store("note", "finished weather task", "daily", memoryMeta{}); err != nil {
		t.Fatalf("store daily: %v", err)
	}
	if err := store.store("other", "unrelated value", "daily", memoryMeta{}); err != nil {
		t.Fatalf("store daily2: %v", err)
	}

	dailyPath := filepath.Join(workspace, "memory", time.Now().Format("2006-01-02")+".md")
	raw, err := os.ReadFile(dailyPath)
	if err != nil {
		t.Fatalf("read daily file: %v", err)
	}
	if !strings.Contains(string(raw), "# Daily Log") {
		t.Fatalf("missing daily header")
	}

	entries, err := store.recall("weather", "", "", 10)
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 recall entry, got %d", len(entries))
	}
	if entries[0].Key != "note" {
		t.Fatalf("expected key note, got %q", entries[0].Key)
	}
}

func TestMarkdownMemoryStoreForgetIsNoop(t *testing.T) {
	store := newMarkdownMemoryStore(t.TempDir())
	removed, err := store.forget("anything")
	if err != nil {
		t.Fatalf("forget should be no-op: %v", err)
	}
	if removed {
		t.Fatalf("forget on markdown backend should return false")
	}
}

func TestMarkdownMemoryStoreRecallLimitZero(t *testing.T) {
	workspace := t.TempDir()
	store := newMarkdownMemoryStore(workspace)
	if err := store.init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := store.store("note", "finished weather task", "daily", memoryMeta{}); err != nil {
		t.Fatalf("store: %v", err)
	}
	entries, err := store.recall("weather", "", "", 0)
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 results when limit=0, got %d", len(entries))
	}
}

func TestMarkdownMemoryStoreGetListCountHealth(t *testing.T) {
	workspace := t.TempDir()
	store := newMarkdownMemoryStore(workspace)
	if !store.healthCheck() {
		t.Fatalf("expected healthCheck=true")
	}
	if err := store.store("core_key", "likes rust", "core", memoryMeta{}); err != nil {
		t.Fatalf("store core: %v", err)
	}
	if err := store.store("daily_key", "weather note", "daily", memoryMeta{}); err != nil {
		t.Fatalf("store daily: %v", err)
	}
	total, err := store.count()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total < 2 {
		t.Fatalf("expected at least 2 entries, got %d", total)
	}
	got, err := store.get("core_key")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil || got.Key != "core_key" {
		t.Fatalf("expected core_key, got %#v", got)
	}
	core, err := store.list("core")
	if err != nil {
		t.Fatalf("list core: %v", err)
	}
	found := false
	for _, e := range core {
		if e.Key == "core_key" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("core_key not found in core list")
	}
}
