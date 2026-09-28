package materials

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/chunking"
	"campusclaw/backend/internal/config"
)

type Indexer interface {
	IndexMaterial(context.Context, int64, int64, chunking.Options) error
}

type Service struct {
	database *sql.DB
	cfg      config.Config
	auth     *auth.Service
	indexer  Indexer
}

type Material struct {
	ID               int64     `json:"id"`
	OriginalFilename string    `json:"original_filename"`
	Uploader         string    `json:"uploader"`
	Mime             string    `json:"mime"`
	SizeBytes        int64     `json:"size_bytes"`
	CreatedAt        time.Time `json:"created_at"`
	IndexStatus      string    `json:"index_status"`
}

type materialRecord struct {
	Material
	classID    int64
	storageKey string
	body       string
}

func New(database *sql.DB, cfg config.Config, authentication *auth.Service) *Service {
	return &Service{database: database, cfg: cfg, auth: authentication}
}

func (s *Service) WithIndexer(indexer Indexer) *Service { s.indexer = indexer; return s }

func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/materials", s.List)
	mux.HandleFunc("POST /api/materials", s.Upload)
	mux.HandleFunc("GET /api/materials/{id}", s.Detail)
	mux.HandleFunc("GET /api/materials/{id}/file", s.Download)
	mux.HandleFunc("POST /api/materials/{id}/reindex", s.Reindex)
}

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	identity, _, ok := s.current(w, r)
	if !ok {
		return
	}
	if hasIdentityOverride(r) {
		auth.WriteError(w, http.StatusForbidden, "identity override rejected")
		return
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if len(search) > 200 {
		auth.WriteError(w, http.StatusBadRequest, "search is too long")
		return
	}
	pattern := "%" + search + "%"
	rows, err := s.database.QueryContext(r.Context(), `SELECT m.id, m.original_filename, COALESCE(u.username, 'system'), m.mime, m.size_bytes, m.created_at,
		(SELECT CASE WHEN SUM(c.index_status = 'failed') > 0 THEN 'failed'
		WHEN SUM(c.index_status = 'pending') > 0 THEN 'pending'
		WHEN COUNT(*) > 0 THEN 'ready' ELSE 'pending' END FROM knowledge_chunks c WHERE c.material_id = m.id)
		FROM materials m LEFT JOIN users u ON u.id = m.uploader_id
		WHERE m.class_id = ? AND m.original_filename LIKE ?
		ORDER BY m.created_at DESC, m.id DESC`, identity.ClassID, pattern)
	if err != nil {
		auth.WriteError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	defer rows.Close()
	items := make([]Material, 0)
	for rows.Next() {
		var item Material
		if err := rows.Scan(&item.ID, &item.OriginalFilename, &item.Uploader, &item.Mime, &item.SizeBytes, &item.CreatedAt, &item.IndexStatus); err != nil {
			auth.WriteError(w, http.StatusServiceUnavailable, "service unavailable")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		auth.WriteError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	auth.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (s *Service) Detail(w http.ResponseWriter, r *http.Request) {
	identity, _, ok := s.current(w, r)
	if !ok {
		return
	}
	if hasIdentityOverride(r) {
		auth.WriteError(w, http.StatusForbidden, "identity override rejected")
		return
	}
	record, status := s.find(r, identity.ClassID)
	if status != 0 {
		materialError(w, status)
		return
	}
	auth.WriteJSON(w, http.StatusOK, struct {
		Material
		Body string `json:"body"`
	}{Material: record.Material, Body: record.body})
}

func (s *Service) Download(w http.ResponseWriter, r *http.Request) {
	identity, _, ok := s.current(w, r)
	if !ok {
		return
	}
	if hasIdentityOverride(r) {
		auth.WriteError(w, http.StatusForbidden, "identity override rejected")
		return
	}
	record, status := s.find(r, identity.ClassID)
	if status != 0 {
		materialError(w, status)
		return
	}
	file, err := os.Open(filepath.Join(s.cfg.UploadDir, record.storageKey))
	if err != nil {
		auth.WriteError(w, http.StatusServiceUnavailable, "file unavailable")
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(record.OriginalFilename))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.Copy(w, file)
}

func (s *Service) Upload(w http.ResponseWriter, r *http.Request) {
	identity, token, ok := s.current(w, r)
	if !ok {
		return
	}
	if identity.Role != "teacher" {
		auth.WriteError(w, http.StatusForbidden, "teacher role required")
		return
	}
	if hasIdentityOverride(r) {
		auth.WriteError(w, http.StatusForbidden, "identity override rejected")
		return
	}
	if !s.auth.ValidCSRF(r, token) {
		auth.WriteError(w, http.StatusForbidden, "invalid csrf token")
		return
	}
	reader, err := r.MultipartReader()
	if err != nil {
		auth.WriteError(w, http.StatusBadRequest, "multipart file required")
		return
	}
	input, err := readUpload(reader, s.cfg.MaxUploadBytes)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		auth.WriteError(w, status, err.Error())
		return
	}
	filename, extension, content := input.filename, input.extension, input.content
	key, err := randomStorageKey()
	if err != nil {
		auth.WriteError(w, http.StatusInternalServerError, "upload unavailable")
		return
	}
	path := filepath.Join(s.cfg.UploadDir, key)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		auth.WriteError(w, http.StatusServiceUnavailable, "upload unavailable")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		auth.WriteError(w, http.StatusServiceUnavailable, "upload unavailable")
		return
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		auth.WriteError(w, http.StatusServiceUnavailable, "upload unavailable")
		return
	}
	if err := file.Close(); err != nil {
		auth.WriteError(w, http.StatusServiceUnavailable, "upload unavailable")
		return
	}
	tx, err := s.database.BeginTx(r.Context(), nil)
	if err != nil {
		auth.WriteError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	defer tx.Rollback()
	mime := "text/plain; charset=utf-8"
	if extension == ".md" {
		mime = "text/markdown; charset=utf-8"
	}
	result, err := tx.ExecContext(r.Context(), "INSERT INTO materials (class_id, uploader_id, original_filename, storage_key, mime, size_bytes) VALUES (?, ?, ?, ?, ?, ?)", identity.ClassID, identity.ID, filename, key, mime, len(content))
	if err != nil {
		auth.WriteError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		auth.WriteError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	if _, err := tx.ExecContext(r.Context(), "INSERT INTO knowledge_entries (material_id, class_id, body) VALUES (?, ?, ?)", id, identity.ClassID, string(content)); err != nil {
		auth.WriteError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	if err := tx.Commit(); err != nil {
		auth.WriteError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	committed = true
	indexStatus := "pending"
	if s.indexer != nil {
		if err := s.indexer.IndexMaterial(r.Context(), id, identity.ClassID, input.options); err != nil {
			indexStatus = "failed"
		} else {
			indexStatus = "ready"
		}
	}
	auth.WriteJSON(w, http.StatusCreated, map[string]any{"id": id, "index_status": indexStatus})
}

func (s *Service) Reindex(w http.ResponseWriter, r *http.Request) {
	identity, token, ok := s.current(w, r)
	if !ok {
		return
	}
	if identity.Role != "teacher" {
		auth.WriteError(w, http.StatusForbidden, "teacher role required")
		return
	}
	if !s.auth.ValidCSRF(r, token) {
		auth.WriteError(w, http.StatusForbidden, "invalid csrf token")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		materialError(w, http.StatusNotFound)
		return
	}
	var exists int64
	if err := s.database.QueryRowContext(r.Context(), "SELECT id FROM materials WHERE id = ? AND class_id = ?", id, identity.ClassID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			materialError(w, http.StatusNotFound)
		} else {
			materialError(w, http.StatusServiceUnavailable)
		}
		return
	}
	var options chunking.Options
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&options); err != nil {
		auth.WriteError(w, http.StatusBadRequest, "invalid chunking options")
		return
	}
	if _, err := options.Normalized(); err != nil {
		auth.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.indexer == nil || s.indexer.IndexMaterial(r.Context(), id, identity.ClassID, options) != nil {
		auth.WriteError(w, http.StatusServiceUnavailable, "indexing failed; original material retained")
		return
	}
	auth.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "index_status": "ready"})
}

func (s *Service) current(w http.ResponseWriter, r *http.Request) (auth.Identity, string, bool) {
	identity, token, status := s.auth.Current(r)
	if status != 0 {
		materialError(w, status)
		return auth.Identity{}, "", false
	}
	return identity, token, true
}

func (s *Service) find(r *http.Request, classID int64) (materialRecord, int) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return materialRecord{}, http.StatusNotFound
	}
	var record materialRecord
	err = s.database.QueryRowContext(r.Context(), `SELECT m.id, m.class_id, m.original_filename, COALESCE(u.username, 'system'), m.storage_key, m.mime, m.size_bytes, m.created_at, k.body,
		(SELECT CASE WHEN SUM(c.index_status = 'failed') > 0 THEN 'failed'
		WHEN SUM(c.index_status = 'pending') > 0 THEN 'pending'
		WHEN COUNT(*) > 0 THEN 'ready' ELSE 'pending' END FROM knowledge_chunks c WHERE c.material_id = m.id)
		FROM materials m LEFT JOIN users u ON u.id = m.uploader_id
		JOIN knowledge_entries k ON k.material_id = m.id
		WHERE m.id = ?`, id).Scan(&record.ID, &record.classID, &record.OriginalFilename, &record.Uploader, &record.storageKey, &record.Mime, &record.SizeBytes, &record.CreatedAt, &record.body, &record.IndexStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return materialRecord{}, http.StatusNotFound
	}
	if err != nil {
		return materialRecord{}, http.StatusServiceUnavailable
	}
	if record.classID != classID {
		return materialRecord{}, http.StatusNotFound
	}
	return record, 0
}

func hasIdentityOverride(r *http.Request) bool {
	query := r.URL.Query()
	return query.Has("class_id") || query.Has("role") || r.Header.Get("X-Class-ID") != "" || r.Header.Get("X-Role") != ""
}

func randomStorageKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func materialError(w http.ResponseWriter, status int) {
	switch status {
	case http.StatusUnauthorized:
		auth.WriteError(w, status, "authentication required")
	case http.StatusNotFound:
		auth.WriteError(w, status, "material not found")
	default:
		auth.WriteError(w, status, "service unavailable")
	}
}
