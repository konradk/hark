package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"hark/internal/ai"
)

const latestSchemaVersion = 4

type migration struct {
	version int
	apply   func(context.Context, *sql.Tx) error
}

var migrations = []migration{
	{version: 1, apply: migrateBaseSchema},
	{version: 2, apply: migrateConversations},
	{version: 3, apply: migrateAttachments},
	{version: 4, apply: migrateDropAttachmentsJSON},
}

func sqliteDSN(path string) string {
	query := url.Values{}
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", "busy_timeout(5000)")
	return (&url.URL{
		Scheme:   "file",
		Path:     path,
		RawQuery: query.Encode(),
	}).String()
}

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode = WAL`); err != nil {
		return fmt.Errorf("enable history WAL mode: %w", err)
	}

	var current int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&current); err != nil {
		return fmt.Errorf("read history schema version: %w", err)
	}
	if current > latestSchemaVersion {
		return fmt.Errorf("history schema version %d is newer than supported version %d", current, latestSchemaVersion)
	}

	for _, item := range migrations {
		if item.version <= current {
			continue
		}
		if err := s.applyMigration(ctx, item); err != nil {
			return err
		}
		current = item.version
	}
	return nil
}

func (s *Store) applyMigration(ctx context.Context, item migration) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin history migration %d: %w", item.version, err)
	}
	defer tx.Rollback()

	if err := item.apply(ctx, tx); err != nil {
		return fmt.Errorf("apply history migration %d: %w", item.version, err)
	}
	if _, err := tx.ExecContext(ctx, `PRAGMA user_version = `+strconv.Itoa(item.version)); err != nil {
		return fmt.Errorf("record history migration %d: %w", item.version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit history migration %d: %w", item.version, err)
	}
	return nil
}

func migrateBaseSchema(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS history_entries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  conversation_id TEXT NOT NULL DEFAULT '',
  prompt TEXT NOT NULL,
  response TEXT NOT NULL,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  attachments_json TEXT NOT NULL DEFAULT '[]',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX IF NOT EXISTS history_entries_created_at_idx
ON history_entries(created_at DESC);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
`)
	return err
}

func migrateConversations(ctx context.Context, tx *sql.Tx) error {
	hasConversationID, err := hasColumn(ctx, tx, "history_entries", "conversation_id")
	if err != nil {
		return err
	}
	if !hasConversationID {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE history_entries ADD COLUMN conversation_id TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add history conversation id: %w", err)
		}
	}

	_, err = tx.ExecContext(ctx, `
UPDATE history_entries
SET conversation_id = 'legacy-' || id
WHERE conversation_id = '';

CREATE INDEX IF NOT EXISTS history_entries_conversation_idx
ON history_entries(conversation_id, id);
`)
	return err
}

func migrateAttachments(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `
CREATE TABLE history_attachments (
  entry_id INTEGER NOT NULL REFERENCES history_entries(id) ON DELETE CASCADE,
  position INTEGER NOT NULL,
  type TEXT NOT NULL,
  path TEXT NOT NULL,
  mime_type TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (entry_id, position)
);

CREATE INDEX history_attachments_path_idx
ON history_attachments(path);
`); err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx, `SELECT id, attachments_json FROM history_entries ORDER BY id`)
	if err != nil {
		return fmt.Errorf("read legacy history attachments: %w", err)
	}
	type legacyAttachments struct {
		entryID     int64
		attachments []ai.Attachment
	}
	var entries []legacyAttachments
	for rows.Next() {
		var entryID int64
		var encoded string
		if err := rows.Scan(&entryID, &encoded); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan legacy history attachments: %w", err)
		}
		var attachments []ai.Attachment
		if err := json.Unmarshal([]byte(encoded), &attachments); err != nil {
			_ = rows.Close()
			return fmt.Errorf("decode legacy history attachments for entry %d: %w", entryID, err)
		}
		entries = append(entries, legacyAttachments{entryID: entryID, attachments: attachments})
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close legacy history attachments: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate legacy history attachments: %w", err)
	}

	for _, entry := range entries {
		for position, attachment := range entry.attachments {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO history_attachments (entry_id, position, type, path, mime_type)
VALUES (?, ?, ?, ?, ?)
`, entry.entryID, position, attachment.Type, attachment.Path, attachment.MIMEType); err != nil {
				return fmt.Errorf("backfill attachment for history entry %d: %w", entry.entryID, err)
			}
		}
	}
	return nil
}

// migrateDropAttachmentsJSON removes the denormalized copy of the attachment
// list. history_attachments has been the cleanup source of truth since
// migration 3; it is now the only one, so the two cannot diverge.
func migrateDropAttachmentsJSON(ctx context.Context, tx *sql.Tx) error {
	exists, err := hasColumn(ctx, tx, "history_entries", "attachments_json")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE history_entries DROP COLUMN attachments_json`); err != nil {
		return fmt.Errorf("drop history attachments_json: %w", err)
	}
	return nil
}

type schemaQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func hasColumn(ctx context.Context, query schemaQuerier, table, column string) (bool, error) {
	rows, err := query.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return false, fmt.Errorf("inspect %s schema: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, fmt.Errorf("scan %s schema: %w", table, err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate %s schema: %w", table, err)
	}
	return false, nil
}
