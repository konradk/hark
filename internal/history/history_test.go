package history

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hark/internal/ai"
)

func TestStoreAddListGetDelete(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	id, err := store.Add(ctx, Entry{
		Prompt:   "prompt",
		Response: "response",
		Provider: "openai",
		Model:    "gpt-test",
		Attachments: []ai.Attachment{{
			Type:     "image",
			Path:     "/tmp/shot.png",
			MIMEType: "image/png",
		}},
	})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	entries, err := store.List(ctx, 10)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Prompt != "prompt" {
		t.Fatalf("unexpected prompt: %q", entries[0].Prompt)
	}
	if len(entries[0].Attachments) != 1 {
		t.Fatalf("unexpected attachments: %#v", entries[0].Attachments)
	}

	entry, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if entry.Response != "response" {
		t.Fatalf("unexpected response: %q", entry.Response)
	}
	if len(entry.Messages) != 2 {
		t.Fatalf("len(entry.Messages) = %d, want 2", len(entry.Messages))
	}

	if _, err := store.DeleteConversation(ctx, id); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := store.Get(ctx, id); err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestStoreGroupsConversationTurns(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	baseTime := time.Date(2026, time.July, 27, 10, 0, 0, 0, time.UTC)
	firstID, err := store.Add(ctx, Entry{
		ConversationID: "chat-one",
		Prompt:         "first question",
		Response:       "first answer",
		Provider:       "openai",
		Model:          "gpt-test",
		CreatedAt:      baseTime,
		Attachments:    []ai.Attachment{{Type: "image", Path: "/tmp/first.png", MIMEType: "image/png"}},
	})
	if err != nil {
		t.Fatalf("Add first turn returned error: %v", err)
	}
	if _, err := store.Add(ctx, Entry{
		ConversationID: "chat-one",
		Prompt:         "follow-up",
		Response:       "follow-up answer",
		Provider:       "openai",
		Model:          "gpt-test",
		CreatedAt:      baseTime.Add(time.Minute),
		Attachments:    []ai.Attachment{{Type: "image", Path: "/tmp/follow-up.png", MIMEType: "image/png"}},
	}); err != nil {
		t.Fatalf("Add follow-up returned error: %v", err)
	}
	if _, err := store.Add(ctx, Entry{
		ConversationID: "chat-two",
		Prompt:         "other question",
		Response:       "other answer",
		Provider:       "openai",
		Model:          "gpt-test",
		CreatedAt:      baseTime.Add(2 * time.Minute),
	}); err != nil {
		t.Fatalf("Add second conversation returned error: %v", err)
	}

	entries, err := store.List(ctx, 10)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	var summary Entry
	for _, entry := range entries {
		if entry.ConversationID == "chat-one" {
			summary = entry
			break
		}
	}
	if summary.ID != firstID {
		t.Fatalf("summary.ID = %d, want first turn ID %d", summary.ID, firstID)
	}
	if summary.Prompt != "first question" || summary.Response != "follow-up answer" {
		t.Fatalf("unexpected conversation summary: %#v", summary)
	}
	if summary.TurnCount != 2 {
		t.Fatalf("summary.TurnCount = %d, want 2", summary.TurnCount)
	}
	if len(summary.Attachments) != 1 || summary.Attachments[0].Path != "/tmp/follow-up.png" {
		t.Fatalf("summary attachments = %#v, want latest turn attachment", summary.Attachments)
	}

	conversation, err := store.Get(ctx, firstID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if len(conversation.Messages) != 4 {
		t.Fatalf("len(conversation.Messages) = %d, want 4", len(conversation.Messages))
	}
	if conversation.Messages[2].Content != "follow-up" || conversation.Messages[3].Content != "follow-up answer" {
		t.Fatalf("unexpected restored messages: %#v", conversation.Messages)
	}
	if len(conversation.Attachments) != 1 || conversation.Attachments[0].Path != "/tmp/follow-up.png" {
		t.Fatalf("conversation attachments = %#v, want latest turn attachment", conversation.Attachments)
	}

	if _, err := store.DeleteConversation(ctx, firstID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := store.Get(ctx, firstID); err == nil {
		t.Fatal("expected the whole conversation to be deleted")
	}
	entries, err = store.List(ctx, 10)
	if err != nil {
		t.Fatalf("List after delete returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].ConversationID != "chat-two" {
		t.Fatalf("unexpected entries after delete: %#v", entries)
	}
}

func TestMigrateBackfillsAttachmentsThenDropsJSONColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	if _, err := db.Exec(`
CREATE TABLE history_entries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  conversation_id TEXT NOT NULL DEFAULT '',
  prompt TEXT NOT NULL,
  response TEXT NOT NULL,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  attachments_json TEXT NOT NULL DEFAULT '[]',
  created_at TEXT NOT NULL
);
CREATE TABLE settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
INSERT INTO history_entries (conversation_id, prompt, response, provider, model, attachments_json, created_at)
VALUES ('chat-1', 'what is this', 'a window', 'openai', 'gpt-test',
  '[{"type":"image","path":"/tmp/region-1.png","mime_type":"image/png"}]',
  '2026-07-27T10:00:00Z');
PRAGMA user_version = 2;
`); err != nil {
		db.Close()
		t.Fatalf("create schema 2 database: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close schema 2 database: %v", err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	entries, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(entries))
	}
	if len(entries[0].Attachments) != 1 || entries[0].Attachments[0].Path != "/tmp/region-1.png" {
		t.Fatalf("attachments were not carried over: %#v", entries[0].Attachments)
	}
	if entries[0].Attachments[0].MIMEType != "image/png" {
		t.Fatalf("attachment mime type lost: %#v", entries[0].Attachments[0])
	}

	conversation, err := store.Get(context.Background(), entries[0].ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if len(conversation.Attachments) != 1 {
		t.Fatalf("Get lost attachments: %#v", conversation.Attachments)
	}

	referenced, err := store.ReferencedAttachmentPaths(context.Background())
	if err != nil {
		t.Fatalf("ReferencedAttachmentPaths returned error: %v", err)
	}
	if len(referenced) != 1 || referenced[0] != "/tmp/region-1.png" {
		t.Fatalf("cleanup lost the attachment reference: %#v", referenced)
	}

	exists, err := hasColumn(context.Background(), store.db, "history_entries", "attachments_json")
	if err != nil {
		t.Fatalf("inspect schema: %v", err)
	}
	if exists {
		t.Fatal("attachments_json column was not dropped")
	}
}

func TestOpenMigratesLegacyEntriesAsSeparateConversations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	if _, err := db.Exec(`
CREATE TABLE history_entries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  prompt TEXT NOT NULL,
  response TEXT NOT NULL,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  attachments_json TEXT NOT NULL DEFAULT '[]',
  created_at TEXT NOT NULL
);
INSERT INTO history_entries (prompt, response, provider, model, attachments_json, created_at)
VALUES
  ('legacy one', 'answer one', 'openai', 'gpt-test', '[]', '2026-07-27T10:00:00Z'),
  ('legacy two', 'answer two', 'openai', 'gpt-test', '[]', '2026-07-27T10:01:00Z');
`); err != nil {
		db.Close()
		t.Fatalf("create legacy database: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open legacy database returned error: %v", err)
	}
	defer store.Close()

	entries, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].ConversationID == entries[1].ConversationID {
		t.Fatalf("legacy entries were grouped together: %#v", entries)
	}
	if entries[0].TurnCount != 1 || entries[1].TurnCount != 1 {
		t.Fatalf("legacy turn counts = %d, %d; want 1, 1", entries[0].TurnCount, entries[1].TurnCount)
	}
}

func TestListEmptyReturnsEmptySlice(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	entries, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if entries == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(entries) != 0 {
		t.Fatalf("len(entries) = %d, want 0", len(entries))
	}
}

func TestSettings(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if _, ok, err := store.GetSetting(ctx, "selected_model"); err != nil || ok {
		t.Fatalf("GetSetting missing = _, %v, %v; want false nil", ok, err)
	}

	if err := store.SetSetting(ctx, "selected_model", "gpt-test"); err != nil {
		t.Fatalf("SetSetting returned error: %v", err)
	}

	value, ok, err := store.GetSetting(ctx, "selected_model")
	if err != nil {
		t.Fatalf("GetSetting returned error: %v", err)
	}
	if !ok || value != "gpt-test" {
		t.Fatalf("GetSetting = %q, %v; want gpt-test true", value, ok)
	}
}

func TestOpenRecordsSchemaVersionAndPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	var version int
	if err := store.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != latestSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, latestSchemaVersion)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("second Migrate returned error: %v", err)
	}
	var foreignKeys int
	if err := store.db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat database: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("database mode = %04o, want 0600", got)
	}
}

func TestOpenRepairsDefaultHistoryDirectoryPermissions(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	directory := filepath.Join(dataHome, "hark")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create permissive history directory: %v", err)
	}
	if err := os.Chmod(directory, 0o755); err != nil {
		t.Fatalf("set permissive history directory mode: %v", err)
	}

	store, err := Open("")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	info, err := os.Stat(directory)
	if err != nil {
		t.Fatalf("stat history directory: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("history directory mode = %04o, want 0700", got)
	}
}

func TestMigrationFailureRollsBackSchemaAndVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	if _, err := db.Exec(`
CREATE TABLE history_entries (
  broken_key INTEGER PRIMARY KEY,
  prompt TEXT NOT NULL,
  response TEXT NOT NULL,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  attachments_json TEXT NOT NULL DEFAULT '[]',
  created_at TEXT NOT NULL
);
PRAGMA user_version = 1;
`); err != nil {
		_ = db.Close()
		t.Fatalf("create broken database: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close broken database: %v", err)
	}

	if _, err := Open(path); err == nil {
		t.Fatal("Open unexpectedly accepted broken migration input")
	}

	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("reopen broken database: %v", err)
	}
	defer db.Close()
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != 1 {
		t.Fatalf("schema version = %d after failed migration, want 1", version)
	}
	rows, err := db.Query(`PRAGMA table_info(history_entries)`)
	if err != nil {
		t.Fatalf("inspect rolled back schema: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan rolled back schema: %v", err)
		}
		if name == "conversation_id" {
			t.Fatal("failed migration left conversation_id behind")
		}
	}
}

func TestCleanupPreservesSharedAttachmentsAndWholeActiveConversations(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	oldTime := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	currentTime := oldTime.Add(60 * 24 * time.Hour)
	shared := "/cache/shared.png"
	oldOnly := "/cache/old.png"
	currentOnly := "/cache/current.png"

	oldID, err := store.Add(ctx, Entry{
		ConversationID: "old",
		Prompt:         "old",
		Response:       "old",
		Provider:       "openai",
		Model:          "test",
		CreatedAt:      oldTime,
		Attachments: []ai.Attachment{
			{Type: "image", Path: shared, MIMEType: "image/png"},
			{Type: "image", Path: oldOnly, MIMEType: "image/png"},
		},
	})
	if err != nil {
		t.Fatalf("Add old entry: %v", err)
	}
	if _, err := store.Add(ctx, Entry{
		ConversationID: "active",
		Prompt:         "first",
		Response:       "first",
		Provider:       "openai",
		Model:          "test",
		CreatedAt:      oldTime,
		Attachments:    []ai.Attachment{{Type: "image", Path: currentOnly, MIMEType: "image/png"}},
	}); err != nil {
		t.Fatalf("Add first active turn: %v", err)
	}
	if _, err := store.Add(ctx, Entry{
		ConversationID: "active",
		Prompt:         "latest",
		Response:       "latest",
		Provider:       "openai",
		Model:          "test",
		CreatedAt:      currentTime,
		Attachments:    []ai.Attachment{{Type: "image", Path: shared, MIMEType: "image/png"}},
	}); err != nil {
		t.Fatalf("Add latest active turn: %v", err)
	}

	result, err := store.PruneBefore(ctx, oldTime.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("PruneBefore returned error: %v", err)
	}
	if result.DeletedEntries != 1 {
		t.Fatalf("DeletedEntries = %d, want 1", result.DeletedEntries)
	}
	if len(result.AttachmentPaths) != 1 || result.AttachmentPaths[0] != oldOnly {
		t.Fatalf("pruned attachment paths = %#v, want only %q", result.AttachmentPaths, oldOnly)
	}
	if _, err := store.Get(ctx, oldID); err == nil {
		t.Fatal("old conversation still exists")
	}
	active, err := store.List(ctx, 10)
	if err != nil {
		t.Fatalf("List active conversations: %v", err)
	}
	if len(active) != 1 || active[0].ConversationID != "active" || active[0].TurnCount != 2 {
		t.Fatalf("active conversation was partially pruned: %#v", active)
	}

	referenced, err := store.ReferencedAttachmentPaths(ctx)
	if err != nil {
		t.Fatalf("ReferencedAttachmentPaths returned error: %v", err)
	}
	if len(referenced) != 2 {
		t.Fatalf("referenced paths = %#v, want two paths", referenced)
	}

	cleared, err := store.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear returned error: %v", err)
	}
	if cleared.DeletedEntries != 2 || len(cleared.AttachmentPaths) != 2 {
		t.Fatalf("Clear result = %#v, want two rows and two paths", cleared)
	}
}
