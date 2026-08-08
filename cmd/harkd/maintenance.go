package main

import (
	"context"
	"fmt"
	"time"

	"hark/internal/history"
	"hark/internal/screenshot"
)

const maintenanceInterval = 6 * time.Hour

func (a *appState) runMaintenance(ctx context.Context) error {
	days, err := a.retentionDays(ctx)
	if err != nil {
		return fmt.Errorf("read retention setting: %w", err)
	}
	if days > 0 {
		result, err := a.history.PruneBefore(ctx, time.Now().AddDate(0, 0, -days))
		if err != nil {
			return err
		}
		if _, err := a.removeManagedScreenshots(result); err != nil {
			return err
		}
	}

	referenced, err := a.history.ReferencedAttachmentPaths(ctx)
	if err != nil {
		return err
	}
	removed, err := a.cleaner.RemoveUnreferenced(referenced, time.Now().Add(-screenshot.UnreferencedGracePeriod))
	if err != nil {
		return fmt.Errorf("clean unreferenced screenshots: %w", err)
	}
	if removed > 0 && a.logger != nil {
		a.logger.Printf("removed %d unreferenced screenshots", removed)
	}
	return nil
}

func (a *appState) maintenanceLoop(ctx context.Context) {
	ticker := time.NewTicker(maintenanceInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.runMaintenance(ctx); err != nil && a.logger != nil {
				a.logger.Printf("maintenance failed: %v", err)
			}
		}
	}
}

func (a *appState) removeManagedScreenshots(result history.CleanupResult) (int, error) {
	removed, err := a.cleaner.RemoveManaged(result.AttachmentPaths)
	if err != nil && a.logger != nil {
		a.logger.Printf("screenshot cleanup failed: %v", err)
	}
	if err != nil {
		return removed, fmt.Errorf("clean history screenshots: %w", err)
	}
	return removed, nil
}
