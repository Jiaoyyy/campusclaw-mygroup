package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"campusclaw/backend/internal/config"
	"github.com/go-sql-driver/mysql"
)

//go:embed migrations/001_init.sql
var migrations embed.FS

func Open(cfg config.Config) (*sql.DB, error) {
	connector, err := mysql.NewConnector(&mysql.Config{
		User:      cfg.DBUser,
		Passwd:    cfg.DBPassword,
		Net:       "tcp",
		Addr:      net.JoinHostPort(cfg.DBHost, strconv.Itoa(cfg.DBPort)),
		DBName:    cfg.DBName,
		ParseTime: true,
		Loc:       time.UTC,
	})
	if err != nil {
		return nil, fmt.Errorf("database configuration: %w", err)
	}
	database := sql.OpenDB(connector)
	database.SetMaxOpenConns(10)
	database.SetMaxIdleConns(10)
	database.SetConnMaxLifetime(30 * time.Minute)
	return database, nil
}

func Migrate(ctx context.Context, database *sql.DB) error {
	schema, err := migrations.ReadFile("migrations/001_init.sql")
	if err != nil {
		return fmt.Errorf("read database schema: %w", err)
	}
	for _, statement := range strings.Split(string(schema), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := database.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply database schema: %w", err)
		}
	}
	return nil
}
