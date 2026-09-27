package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"campusclaw/backend/internal/config"
	"golang.org/x/crypto/bcrypt"
)

type seedMaterial struct {
	key      string
	classID  int64
	uploader any
	filename string
	content  string
}

func Seed(ctx context.Context, database *sql.DB, cfg config.Config) (err error) {
	if err := os.MkdirAll(cfg.UploadDir, 0o700); err != nil {
		return fmt.Errorf("prepare upload directory: %w", err)
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin seed transaction: %w", err)
	}
	var newFiles []string
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			for _, path := range newFiles {
				_ = os.Remove(path)
			}
		}
	}()

	classA, err := ensureClass(ctx, tx, "A")
	if err != nil {
		return err
	}
	classB, err := ensureClass(ctx, tx, "B")
	if err != nil {
		return err
	}
	teacherA, err := ensureUser(ctx, tx, "teacher_a", "teacher", classA, cfg.SeedTeacherAPassword)
	if err != nil {
		return err
	}
	if _, err = ensureUser(ctx, tx, "student_a1", "student", classA, cfg.SeedStudentA1Password); err != nil {
		return err
	}
	if _, err = ensureUser(ctx, tx, "student_b1", "student", classB, cfg.SeedStudentB1Password); err != nil {
		return err
	}

	for _, material := range []seedMaterial{
		{key: "class-a-introduction", classID: classA, uploader: teacherA, filename: "A班课程提纲.txt", content: "A班课程提纲\n本班学习认证、授权与材料入库。\n"},
		{key: "class-b-lab", classID: classB, uploader: nil, filename: "B班实验须知.md", content: "# B班实验须知\n\n本班实验材料仅供B班查看。\n"},
	} {
		if err = ensureMaterial(ctx, tx, cfg.UploadDir, material, &newFiles); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit seed transaction: %w", err)
	}
	return nil
}

func ensureClass(ctx context.Context, tx *sql.Tx, name string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, "SELECT id FROM classes WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("query seed class: %w", err)
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO classes (name) VALUES (?)", name)
	if err != nil {
		return 0, fmt.Errorf("insert seed class: %w", err)
	}
	return result.LastInsertId()
}

func ensureUser(ctx context.Context, tx *sql.Tx, username, role string, classID int64, password string) (int64, error) {
	var id, existingClass int64
	var existingRole string
	err := tx.QueryRowContext(ctx, "SELECT id, role, class_id FROM users WHERE username = ?", username).Scan(&id, &existingRole, &existingClass)
	if err == nil {
		if existingRole != role || existingClass != classID {
			return 0, fmt.Errorf("seed user %s has unexpected role or class", username)
		}
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("query seed user: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("hash seed password: %w", err)
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO users (username, password_hash, role, class_id) VALUES (?, ?, ?, ?)", username, string(hash), role, classID)
	if err != nil {
		return 0, fmt.Errorf("insert seed user: %w", err)
	}
	return result.LastInsertId()
}

func ensureMaterial(ctx context.Context, tx *sql.Tx, uploadDir string, material seedMaterial, newFiles *[]string) error {
	digest := sha256.Sum256([]byte("campusclaw-seed:v1:" + material.key))
	storageKey := hex.EncodeToString(digest[:])
	var id, existingClass int64
	err := tx.QueryRowContext(ctx, "SELECT id, class_id FROM materials WHERE storage_key = ?", storageKey).Scan(&id, &existingClass)
	if err == nil {
		if existingClass != material.classID {
			return fmt.Errorf("seed material has unexpected class")
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("query seed material: %w", err)
	}
	path := filepath.Join(uploadDir, storageKey)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create seed file: %w", err)
	}
	*newFiles = append(*newFiles, path)
	if _, err = file.WriteString(material.content); err != nil {
		_ = file.Close()
		return fmt.Errorf("write seed file: %w", err)
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("close seed file: %w", err)
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO materials (class_id, uploader_id, original_filename, storage_key, mime, size_bytes) VALUES (?, ?, ?, ?, ?, ?)", material.classID, material.uploader, material.filename, storageKey, "text/plain; charset=utf-8", len(material.content))
	if err != nil {
		return fmt.Errorf("insert seed material: %w", err)
	}
	id, err = result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read seed material ID: %w", err)
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO knowledge_entries (material_id, class_id, body) VALUES (?, ?, ?)", id, material.classID, material.content); err != nil {
		return fmt.Errorf("insert seed knowledge entry: %w", err)
	}
	return nil
}
