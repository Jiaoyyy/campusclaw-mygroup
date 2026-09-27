package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"campusclaw/backend/internal/config"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

func TestSeedIdempotence(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run the MySQL integration test")
	}
	database, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	if err := database.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, database); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		UploadDir:             t.TempDir(),
		SeedTeacherAPassword:  "integration-teacher-password",
		SeedStudentA1Password: "integration-shared-student-password",
		SeedStudentB1Password: "integration-shared-student-password",
	}
	if err := Seed(ctx, database, cfg); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	var teacherID, classID int64
	if err := database.QueryRowContext(ctx, "SELECT id, class_id FROM users WHERE username = 'teacher_a'").Scan(&teacherID, &classID); err != nil {
		t.Fatal(err)
	}
	uploadKey := strings.Repeat("f", 64)
	uploadBody := "teacher upload must survive reseeding"
	if err := os.WriteFile(filepath.Join(cfg.UploadDir, uploadKey), []byte(uploadBody), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := database.ExecContext(ctx, "INSERT INTO materials (class_id, uploader_id, original_filename, storage_key, mime, size_bytes) VALUES (?, ?, ?, ?, ?, ?)", classID, teacherID, "teacher.txt", uploadKey, "text/plain", len(uploadBody))
	if err != nil {
		t.Fatal(err)
	}
	uploadID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, "INSERT INTO knowledge_entries (material_id, class_id, body) VALUES (?, ?, ?)", uploadID, classID, uploadBody); err != nil {
		t.Fatal(err)
	}
	if err := Seed(ctx, database, cfg); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	for table, want := range map[string]int{"classes": 2, "users": 3, "materials": 3, "knowledge_entries": 3} {
		var got int
		if err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s count = %d, want %d", table, got, want)
		}
	}
	var hash string
	if err := database.QueryRowContext(ctx, "SELECT password_hash FROM users WHERE username = 'teacher_a'").Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash == cfg.SeedTeacherAPassword || bcrypt.CompareHashAndPassword([]byte(hash), []byte(cfg.SeedTeacherAPassword)) != nil {
		t.Fatal("seed password was not stored as a matching bcrypt hash")
	}
	var studentAHash, studentBHash string
	if err := database.QueryRowContext(ctx, "SELECT password_hash FROM users WHERE username = 'student_a1'").Scan(&studentAHash); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, "SELECT password_hash FROM users WHERE username = 'student_b1'").Scan(&studentBHash); err != nil {
		t.Fatal(err)
	}
	if studentAHash == studentBHash || bcrypt.CompareHashAndPassword([]byte(studentAHash), []byte(cfg.SeedStudentA1Password)) != nil || bcrypt.CompareHashAndPassword([]byte(studentBHash), []byte(cfg.SeedStudentB1Password)) != nil {
		t.Fatal("equal test passwords did not receive independent bcrypt salts")
	}
	var savedBody string
	if err := database.QueryRowContext(ctx, "SELECT body FROM knowledge_entries WHERE material_id = ?", uploadID).Scan(&savedBody); err != nil || savedBody != uploadBody {
		t.Fatalf("teacher upload was changed by reseeding: %v", err)
	}
	rows, err := database.QueryContext(ctx, "SELECT m.storage_key, k.body FROM materials m JOIN knowledge_entries k ON k.material_id = m.id AND k.class_id = m.class_id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var key, body string
		if err := rows.Scan(&key, &body); err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(filepath.Join(cfg.UploadDir, key))
		if err != nil || string(content) != body {
			t.Fatalf("seed file and knowledge body differ: %v", err)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
