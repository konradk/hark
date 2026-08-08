package history

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hark/internal/ai"
	"hark/internal/settings"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Entry struct {
	ID             int64           `json:"id"`
	ConversationID string          `json:"conversation_id"`
	Prompt         string          `json:"prompt"`
	Response       string          `json:"response"`
	Provider       string          `json:"provider"`
	Model          string          `json:"model"`
	Attachments    []ai.Attachment `json:"attachments,omitempty"`
	Messages       []ai.Message    `json:"messages,omitempty"`
	TurnCount      int             `json:"turn_count,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

func DefaultPath() string {
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		return filepath.Join(dataHome, "hark", "history.db")
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", "history.db")
	}

	return filepath.Join(home, ".local", "share", "hark", "history.db")
}

func Open(path string) (*Store, error) {
	usesDefaultPath := path == ""
	if path == "" {
		path = DefaultPath()
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create history directory: %w", err)
	}
	if usesDefaultPath {
		if err := os.Chmod(directory, 0o700); err != nil {
			return nil, fmt.Errorf("set history directory permissions: %w", err)
		}
	}

	db, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		return nil, fmt.Errorf("open history database: %w", err)
	}
	db.SetMaxOpenConns(4)

	store := &Store{db: db}
	if err := store.Migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set history database permissions: %w", err)
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Add(ctx context.Context, entry Entry) (int64, error) {
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	if entry.ConversationID == "" {
		entry.ConversationID = fmt.Sprintf("chat-%d", entry.CreatedAt.UnixNano())
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin history insert: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
INSERT INTO history_entries (conversation_id, prompt, response, provider, model, created_at)
VALUES (?, ?, ?, ?, ?, ?)
`, entry.ConversationID, entry.Prompt, entry.Response, entry.Provider, entry.Model, entry.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("insert history entry: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read history insert id: %w", err)
	}
	for position, attachment := range entry.Attachments {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO history_attachments (entry_id, position, type, path, mime_type)
VALUES (?, ?, ?, ?, ?)
`, id, position, attachment.Type, attachment.Path, attachment.MIMEType); err != nil {
			return 0, fmt.Errorf("insert history attachment: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit history insert: %w", err)
	}
	return id, nil
}

func (s *Store) List(ctx context.Context, limit int) ([]Entry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT
  first_entry.id,
  first_entry.conversation_id,
  first_entry.prompt,
  latest_entry.response,
  latest_entry.provider,
  latest_entry.model,
  latest_entry.created_at,
  grouped.turn_count
FROM (
  SELECT conversation_id, MIN(id) AS first_id, MAX(id) AS latest_id, COUNT(*) AS turn_count
  FROM history_entries
  GROUP BY conversation_id
) AS grouped
JOIN history_entries AS first_entry ON first_entry.id = grouped.first_id
JOIN history_entries AS latest_entry ON latest_entry.id = grouped.latest_id
ORDER BY latest_entry.created_at DESC, latest_entry.id DESC
LIMIT ?
`, limit)
	if err != nil {
		return nil, fmt.Errorf("list history entries: %w", err)
	}
	defer rows.Close()

	entries := []Entry{}
	for rows.Next() {
		entry, err := scanSummary(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate history entries: %w", err)
	}
	if err := loadLatestConversationAttachments(ctx, s.db, entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *Store) Get(ctx context.Context, id int64) (Entry, error) {
	var conversationID string
	if err := s.db.QueryRowContext(ctx, `
SELECT conversation_id
FROM history_entries
WHERE id = ?
`, id).Scan(&conversationID); err != nil {
		if err == sql.ErrNoRows {
			return Entry{}, fmt.Errorf("history entry %d not found", id)
		}
		return Entry{}, fmt.Errorf("resolve history conversation: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, conversation_id, prompt, response, provider, model, created_at
FROM history_entries
WHERE conversation_id = ?
ORDER BY id ASC
`, conversationID)
	if err != nil {
		return Entry{}, fmt.Errorf("load history conversation: %w", err)
	}
	defer rows.Close()

	var conversation Entry
	conversation.Messages = []ai.Message{}
	for rows.Next() {
		turn, err := scanEntry(rows)
		if err != nil {
			return Entry{}, err
		}
		if conversation.TurnCount == 0 {
			conversation = turn
			conversation.Messages = []ai.Message{}
		}
		conversation.Response = turn.Response
		conversation.Provider = turn.Provider
		conversation.Model = turn.Model
		conversation.CreatedAt = turn.CreatedAt
		conversation.Messages = append(conversation.Messages,
			ai.Message{Role: "user", Content: turn.Prompt},
			ai.Message{Role: "assistant", Content: turn.Response},
		)
		conversation.TurnCount++
	}
	if err := rows.Err(); err != nil {
		return Entry{}, fmt.Errorf("iterate history conversation: %w", err)
	}
	if conversation.TurnCount == 0 {
		return Entry{}, fmt.Errorf("history entry %d not found", id)
	}
	loaded := []Entry{conversation}
	if err := loadLatestConversationAttachments(ctx, s.db, loaded); err != nil {
		return Entry{}, err
	}
	return loaded[0], nil
}

func (s *Store) GetSetting(ctx context.Context, key settings.Key) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get setting %q: %w", key, err)
	}
	return value, true, nil
}

func (s *Store) SetSetting(ctx context.Context, key settings.Key, value string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO settings (key, value, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET
  value = excluded.value,
  updated_at = excluded.updated_at
`, key, value, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanEntry(row scanner) (Entry, error) {
	var entry Entry
	var createdAt string
	if err := row.Scan(
		&entry.ID,
		&entry.ConversationID,
		&entry.Prompt,
		&entry.Response,
		&entry.Provider,
		&entry.Model,
		&createdAt,
	); err != nil {
		return Entry{}, err
	}
	if err := decodeCreatedAt(&entry, createdAt); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

func scanSummary(row scanner) (Entry, error) {
	var entry Entry
	var createdAt string
	if err := row.Scan(
		&entry.ID,
		&entry.ConversationID,
		&entry.Prompt,
		&entry.Response,
		&entry.Provider,
		&entry.Model,
		&createdAt,
		&entry.TurnCount,
	); err != nil {
		return Entry{}, err
	}
	if err := decodeCreatedAt(&entry, createdAt); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

func decodeCreatedAt(entry *Entry, createdAt string) error {
	timestamp, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return fmt.Errorf("decode history timestamp: %w", err)
	}
	entry.CreatedAt = timestamp
	return nil
}

func loadLatestConversationAttachments(ctx context.Context, query rowQuerier, entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}
	conversationIDs := make([]any, 0, len(entries))
	for _, entry := range entries {
		conversationIDs = append(conversationIDs, entry.ConversationID)
	}

	rows, err := query.QueryContext(ctx, `
SELECT entry.conversation_id, attachment.type, attachment.path, attachment.mime_type
FROM history_entries AS entry
JOIN history_attachments AS attachment ON attachment.entry_id = entry.id
WHERE entry.id IN (
  SELECT MAX(candidate.id)
  FROM history_entries AS candidate
  JOIN history_attachments AS present ON present.entry_id = candidate.id
  WHERE candidate.conversation_id IN (`+placeholders(len(conversationIDs))+`)
  GROUP BY candidate.conversation_id
)
ORDER BY entry.id, attachment.position
`, conversationIDs...)
	if err != nil {
		return fmt.Errorf("load history attachments: %w", err)
	}
	defer rows.Close()

	byConversation := make(map[string][]ai.Attachment, len(conversationIDs))
	for rows.Next() {
		var conversationID string
		var attachment ai.Attachment
		if err := rows.Scan(&conversationID, &attachment.Type, &attachment.Path, &attachment.MIMEType); err != nil {
			return fmt.Errorf("scan history attachment: %w", err)
		}
		byConversation[conversationID] = append(byConversation[conversationID], attachment)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate history attachments: %w", err)
	}

	for index := range entries {
		entries[index].Attachments = byConversation[entries[index].ConversationID]
	}
	return nil
}

func placeholders(count int) string {
	return strings.TrimPrefix(strings.Repeat(",?", count), ",")
}
