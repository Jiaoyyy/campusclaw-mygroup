package materials

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func CleanupOrphans(ctx context.Context, database *sql.DB, uploadDir string, grace time.Duration) error {
	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		return fmt.Errorf("scan upload directory: %w", err)
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect upload file: %w", err)
		}
		if time.Since(info.ModTime()) < grace {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, ".tmp-") {
			var count int
			if err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM materials WHERE storage_key = ?", name).Scan(&count); err != nil {
				return fmt.Errorf("check upload reference: %w", err)
			}
			if count != 0 {
				continue
			}
		}
		if err := os.Remove(filepath.Join(uploadDir, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove orphan upload: %w", err)
		}
	}
	return nil
}
