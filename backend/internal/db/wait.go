package db

import (
	"context"
	"database/sql"
	"time"
)

func WaitReady(ctx context.Context, database *sql.DB) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if err := database.PingContext(ctx); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
