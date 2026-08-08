package history

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type CleanupResult struct {
	DeletedEntries  int64    `json:"deleted_entries"`
	AttachmentPaths []string `json:"attachment_paths,omitempty"`
}

func (s *Store) DeleteConversation(ctx context.Context, id int64) (CleanupResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CleanupResult{}, fmt.Errorf("begin history delete: %w", err)
	}
	defer tx.Rollback()

	var conversationID string
	if err := tx.QueryRowContext(ctx, `
SELECT conversation_id
FROM history_entries
WHERE id = ?
`, id).Scan(&conversationID); err != nil {
		if err == sql.ErrNoRows {
			return CleanupResult{}, fmt.Errorf("history entry %d not found", id)
		}
		return CleanupResult{}, fmt.Errorf("resolve history conversation: %w", err)
	}

	candidates, err := attachmentPathsForConversation(ctx, tx, conversationID)
	if err != nil {
		return CleanupResult{}, err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM history_entries WHERE conversation_id = ?`, conversationID)
	if err != nil {
		return CleanupResult{}, fmt.Errorf("delete history conversation: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return CleanupResult{}, fmt.Errorf("read deleted history rows: %w", err)
	}
	orphans, err := orphanedPaths(ctx, tx, candidates)
	if err != nil {
		return CleanupResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return CleanupResult{}, fmt.Errorf("commit history delete: %w", err)
	}
	return CleanupResult{DeletedEntries: deleted, AttachmentPaths: orphans}, nil
}

func (s *Store) Clear(ctx context.Context) (CleanupResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CleanupResult{}, fmt.Errorf("begin history clear: %w", err)
	}
	defer tx.Rollback()

	paths, err := allAttachmentPaths(ctx, tx)
	if err != nil {
		return CleanupResult{}, err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM history_entries`)
	if err != nil {
		return CleanupResult{}, fmt.Errorf("clear history: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return CleanupResult{}, fmt.Errorf("read cleared history rows: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return CleanupResult{}, fmt.Errorf("commit history clear: %w", err)
	}
	return CleanupResult{DeletedEntries: deleted, AttachmentPaths: paths}, nil
}

func (s *Store) PruneBefore(ctx context.Context, cutoff time.Time) (CleanupResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CleanupResult{}, fmt.Errorf("begin history prune: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
SELECT conversation_id
FROM history_entries
GROUP BY conversation_id
HAVING MAX(created_at) < ?
`, cutoff.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return CleanupResult{}, fmt.Errorf("list expired history conversations: %w", err)
	}
	var conversationIDs []string
	for rows.Next() {
		var conversationID string
		if err := rows.Scan(&conversationID); err != nil {
			_ = rows.Close()
			return CleanupResult{}, fmt.Errorf("scan expired history conversation: %w", err)
		}
		conversationIDs = append(conversationIDs, conversationID)
	}
	if err := rows.Close(); err != nil {
		return CleanupResult{}, fmt.Errorf("close expired history conversations: %w", err)
	}
	if err := rows.Err(); err != nil {
		return CleanupResult{}, fmt.Errorf("iterate expired history conversations: %w", err)
	}

	var deleted int64
	var candidates []string
	for _, conversationID := range conversationIDs {
		paths, err := attachmentPathsForConversation(ctx, tx, conversationID)
		if err != nil {
			return CleanupResult{}, err
		}
		candidates = append(candidates, paths...)
		result, err := tx.ExecContext(ctx, `DELETE FROM history_entries WHERE conversation_id = ?`, conversationID)
		if err != nil {
			return CleanupResult{}, fmt.Errorf("delete expired history conversation: %w", err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return CleanupResult{}, fmt.Errorf("read pruned history rows: %w", err)
		}
		deleted += count
	}
	orphans, err := orphanedPaths(ctx, tx, candidates)
	if err != nil {
		return CleanupResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return CleanupResult{}, fmt.Errorf("commit history prune: %w", err)
	}
	return CleanupResult{DeletedEntries: deleted, AttachmentPaths: orphans}, nil
}

func (s *Store) ReferencedAttachmentPaths(ctx context.Context) ([]string, error) {
	return allAttachmentPaths(ctx, s.db)
}

type rowQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func attachmentPathsForConversation(ctx context.Context, query rowQuerier, conversationID string) ([]string, error) {
	rows, err := query.QueryContext(ctx, `
SELECT DISTINCT attachment.path
FROM history_attachments AS attachment
JOIN history_entries AS entry ON entry.id = attachment.entry_id
WHERE entry.conversation_id = ?
`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list history attachment paths: %w", err)
	}
	return scanPaths(rows)
}

func allAttachmentPaths(ctx context.Context, query rowQuerier) ([]string, error) {
	rows, err := query.QueryContext(ctx, `SELECT DISTINCT path FROM history_attachments`)
	if err != nil {
		return nil, fmt.Errorf("list history attachment paths: %w", err)
	}
	return scanPaths(rows)
}

func scanPaths(rows *sql.Rows) ([]string, error) {
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("scan history attachment path: %w", err)
		}
		if path != "" {
			paths = append(paths, path)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate history attachment paths: %w", err)
	}
	return paths, nil
}

func orphanedPaths(ctx context.Context, query rowQuerier, candidates []string) ([]string, error) {
	seen := make(map[string]struct{}, len(candidates))
	var orphans []string
	for _, path := range candidates {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		var references int
		if err := query.QueryRowContext(ctx, `SELECT COUNT(*) FROM history_attachments WHERE path = ?`, path).Scan(&references); err != nil {
			return nil, fmt.Errorf("count history attachment references: %w", err)
		}
		if references == 0 {
			orphans = append(orphans, path)
		}
	}
	return orphans, nil
}
