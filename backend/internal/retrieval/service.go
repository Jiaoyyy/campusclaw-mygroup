package retrieval

import (
	"database/sql"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"campusclaw/backend/internal/auth"
)

const (
	maxQueryRunes = 100
	maxHits       = 20
	chunkRunes    = 400
	contextRunes  = 60
)

type Hit struct {
	MaterialID int64  `json:"material_id"`
	Title      string `json:"title"`
	ChunkIndex int    `json:"chunk_index"`
	Start      int    `json:"start_offset"`
	End        int    `json:"end_offset"`
	Snippet    string `json:"snippet"`
}

type Service struct {
	database *sql.DB
	auth     *auth.Service
}

func New(database *sql.DB, authentication *auth.Service) *Service {
	return &Service{database: database, auth: authentication}
}

func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/retrieval", s.Search)
}

func (s *Service) Search(w http.ResponseWriter, r *http.Request) {
	identity, _, status := s.auth.Current(r)
	if status != 0 {
		if status == http.StatusUnauthorized {
			auth.WriteError(w, status, "authentication required")
		} else {
			auth.WriteError(w, status, "service unavailable")
		}
		return
	}
	query := r.URL.Query()
	if query.Has("class_id") || query.Has("role") || r.Header.Get("X-Class-ID") != "" || r.Header.Get("X-Role") != "" {
		auth.WriteError(w, http.StatusForbidden, "identity override rejected")
		return
	}
	term := strings.TrimSpace(query.Get("q"))
	if term == "" || utf8.RuneCountInString(term) > maxQueryRunes {
		auth.WriteError(w, http.StatusBadRequest, "query must be 1-100 characters")
		return
	}
	rows, err := s.database.QueryContext(r.Context(), `SELECT m.id, m.original_filename, k.body
		FROM knowledge_entries k JOIN materials m ON m.id = k.material_id AND m.class_id = k.class_id
		WHERE k.class_id = ? ORDER BY m.created_at DESC, m.id DESC`, identity.ClassID)
	if err != nil {
		auth.WriteError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	defer rows.Close()
	hits := make([]Hit, 0)
	for rows.Next() {
		var materialID int64
		var title, body string
		if err := rows.Scan(&materialID, &title, &body); err != nil {
			auth.WriteError(w, http.StatusServiceUnavailable, "service unavailable")
			return
		}
		for _, hit := range findHits(body, term, materialID, title, maxHits-len(hits)) {
			hits = append(hits, hit)
		}
		if len(hits) == maxHits {
			break
		}
	}
	if err := rows.Err(); err != nil {
		auth.WriteError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	auth.WriteJSON(w, http.StatusOK, map[string]any{"hits": hits})
}

func findHits(body, term string, materialID int64, title string, limit int) []Hit {
	if limit <= 0 || term == "" {
		return nil
	}
	original := []rune(body)
	lowerBody := strings.Map(unicode.ToLower, body)
	lowerTerm := strings.Map(unicode.ToLower, term)
	termRunes := utf8.RuneCountInString(term)
	hits := make([]Hit, 0)
	fromByte, fromRune := 0, 0
	for len(hits) < limit {
		relativeByte := strings.Index(lowerBody[fromByte:], lowerTerm)
		if relativeByte < 0 {
			break
		}
		startByte := fromByte + relativeByte
		start := fromRune + utf8.RuneCountInString(lowerBody[fromByte:startByte])
		end := start + termRunes
		snippetStart := max(0, start-contextRunes)
		snippetEnd := min(len(original), end+contextRunes)
		hits = append(hits, Hit{
			MaterialID: materialID, Title: title, ChunkIndex: start/chunkRunes + 1,
			Start: start, End: end, Snippet: string(original[snippetStart:snippetEnd]),
		})
		fromByte = startByte + len(lowerTerm)
		fromRune = end
	}
	return hits
}
