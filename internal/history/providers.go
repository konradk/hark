package history

import (
	"context"
	"fmt"
)

// Provider is a user-defined OpenAI-compatible provider managed from the
// settings panel. It is separate from the providers declared in config.lua.
type Provider struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	BaseURL string `json:"base_url"`
}

// Model is a user-defined model that references a Provider by ID.
type Model struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Provider string `json:"provider"`
}

func (s *Store) ListProviders(ctx context.Context) ([]Provider, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, label, base_url FROM providers ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()

	providers := []Provider{}
	for rows.Next() {
		var provider Provider
		if err := rows.Scan(&provider.ID, &provider.Label, &provider.BaseURL); err != nil {
			return nil, fmt.Errorf("scan provider: %w", err)
		}
		providers = append(providers, provider)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate providers: %w", err)
	}
	return providers, nil
}

func (s *Store) UpsertProvider(ctx context.Context, provider Provider) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO providers (id, label, base_url)
VALUES (?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  label = excluded.label,
  base_url = excluded.base_url
`, provider.ID, provider.Label, provider.BaseURL)
	if err != nil {
		return fmt.Errorf("upsert provider %q: %w", provider.ID, err)
	}
	return nil
}

func (s *Store) DeleteProvider(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM providers WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete provider %q: %w", id, err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM models WHERE provider = ?`, id); err != nil {
		return fmt.Errorf("delete provider models %q: %w", id, err)
	}
	return nil
}

func (s *Store) ListModels(ctx context.Context) ([]Model, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, label, provider FROM models ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer rows.Close()

	models := []Model{}
	for rows.Next() {
		var model Model
		if err := rows.Scan(&model.ID, &model.Label, &model.Provider); err != nil {
			return nil, fmt.Errorf("scan model: %w", err)
		}
		models = append(models, model)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate models: %w", err)
	}
	return models, nil
}

func (s *Store) UpsertModel(ctx context.Context, model Model) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO models (id, label, provider)
VALUES (?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  label = excluded.label,
  provider = excluded.provider
`, model.ID, model.Label, model.Provider)
	if err != nil {
		return fmt.Errorf("upsert model %q: %w", model.ID, err)
	}
	return nil
}

func (s *Store) DeleteModel(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM models WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete model %q: %w", id, err)
	}
	return nil
}
