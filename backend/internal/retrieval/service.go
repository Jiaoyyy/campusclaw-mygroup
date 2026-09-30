package retrieval

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"campusclaw/backend/internal/auth"
)

const (
	maxQueryRunes = 100
	maxHits       = 20
	noEvidence    = "资料中未找到相关内容"
)

type Hit struct {
	ChunkID        int64   `json:"chunk_id"`
	MaterialID     int64   `json:"material_id"`
	Title          string  `json:"title"`
	ChunkIndex     int     `json:"chunk_index"`
	Start          int     `json:"start_offset"`
	End            int     `json:"end_offset"`
	Snippet        string  `json:"snippet"`
	Score          float64 `json:"score"`
	CitationNumber int     `json:"citation_number,omitempty"`
	ChunkText      string  `json:"-"`
}

type Service struct {
	database      *sql.DB
	auth          *auth.Service
	embedder      Embedder
	vectors       VectorStore
	answerer      Answerer
	vectorEnabled bool
	answerEnabled bool
}

func New(database *sql.DB, authentication *auth.Service, embedder Embedder, vectors VectorStore, answerer Answerer, vectorEnabled, answerEnabled bool) *Service {
	return &Service{database: database, auth: authentication, embedder: embedder, vectors: vectors, answerer: answerer, vectorEnabled: vectorEnabled, answerEnabled: answerEnabled}
}

func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/retrieval", s.Search)
	mux.HandleFunc("GET /api/retrieval/capabilities", s.Capabilities)
	mux.HandleFunc("POST /api/ask", s.Ask)
}

func (s *Service) Capabilities(w http.ResponseWriter, r *http.Request) {
	if _, _, status := s.auth.Current(r); status != 0 {
		writeStatus(w, status)
		return
	}
	defaultMode := "keyword"
	if s.vectorEnabled {
		defaultMode = "hybrid"
	}
	auth.WriteJSON(w, http.StatusOK, map[string]any{
		"vector_enabled": s.vectorEnabled, "answer_enabled": s.answerEnabled, "default_mode": defaultMode,
	})
}

func (s *Service) Search(w http.ResponseWriter, r *http.Request) {
	identity, _, status := s.auth.Current(r)
	if status != 0 {
		writeStatus(w, status)
		return
	}
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "keyword"
		if s.vectorEnabled {
			mode = "hybrid"
		}
	}
	if !validQuery(term) || !validMode(mode) {
		auth.WriteError(w, http.StatusBadRequest, "invalid retrieval query or mode")
		return
	}
	if mode != "keyword" && !s.vectorEnabled {
		writeStatus(w, http.StatusServiceUnavailable)
		return
	}
	hits, err := s.search(r.Context(), identity.ClassID, term, mode, maxHits)
	if err != nil {
		writeStatus(w, http.StatusServiceUnavailable)
		return
	}
	message := ""
	if len(hits) == 0 {
		message = noEvidence
	}
	auth.WriteJSON(w, http.StatusOK, map[string]any{"hits": hits, "mode": mode, "message": message})
}

func (s *Service) Ask(w http.ResponseWriter, r *http.Request) {
	identity, token, status := s.auth.Current(r)
	if status != 0 {
		writeStatus(w, status)
		return
	}
	if !s.auth.ValidCSRF(r, token) {
		auth.WriteError(w, http.StatusForbidden, "invalid csrf token")
		return
	}
	if !s.answerEnabled {
		writeStatus(w, http.StatusServiceUnavailable)
		return
	}
	var input struct {
		Question string `json:"question"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input); err != nil {
		auth.WriteError(w, http.StatusBadRequest, "invalid question")
		return
	}
	input.Question = strings.TrimSpace(input.Question)
	if !validQuery(input.Question) {
		auth.WriteError(w, http.StatusBadRequest, "invalid question")
		return
	}
	hits, err := s.search(r.Context(), identity.ClassID, input.Question, "hybrid", 4)
	if err != nil {
		writeStatus(w, http.StatusServiceUnavailable)
		return
	}
	if len(hits) == 0 {
		auth.WriteJSON(w, http.StatusOK, map[string]any{"answer": noEvidence, "citations": []Hit{}})
		return
	}
	answer, err := s.answerer.Answer(r.Context(), input.Question, hits)
	if err != nil {
		writeStatus(w, http.StatusServiceUnavailable)
		return
	}
	citations := citedHits(answer, hits)
	if len(citations) == 0 {
		writeStatus(w, http.StatusServiceUnavailable)
		return
	}
	auth.WriteJSON(w, http.StatusOK, map[string]any{"answer": answer, "citations": citations})
}

func citedHits(answer string, hits []Hit) []Hit {
	used := make(map[int]bool)
	for _, match := range citationPattern.FindAllStringSubmatch(answer, -1) {
		number, err := strconv.Atoi(match[1])
		if err == nil && number >= 1 && number <= len(hits) {
			used[number] = true
		}
	}
	citations := make([]Hit, 0, len(used))
	for index, hit := range hits {
		if used[index+1] {
			hit.CitationNumber = index + 1
			citations = append(citations, hit)
		}
	}
	return citations
}

func validQuery(query string) bool {
	return query != "" && utf8.RuneCountInString(query) <= maxQueryRunes
}
func validMode(mode string) bool { return mode == "keyword" || mode == "vector" || mode == "hybrid" }
func writeStatus(w http.ResponseWriter, status int) {
	if status == http.StatusUnauthorized {
		auth.WriteError(w, status, "authentication required")
	} else {
		auth.WriteError(w, status, "service unavailable")
	}
}

func (s *Service) search(ctx context.Context, classID int64, term, mode string, limit int) ([]Hit, error) {
	switch mode {
	case "keyword":
		return s.keyword(ctx, classID, term, limit)
	case "vector":
		return s.vector(ctx, classID, term, limit)
	case "hybrid":
		keywords, err := s.keyword(ctx, classID, term, maxHits)
		if err != nil {
			return nil, err
		}
		vectors, err := s.vector(ctx, classID, term, maxHits)
		if err != nil {
			return nil, err
		}
		return fuse(keywords, vectors, limit), nil
	default:
		return nil, errors.New("unsupported retrieval mode")
	}
}

func (s *Service) keyword(ctx context.Context, classID int64, term string, limit int) ([]Hit, error) {
	rows, err := s.database.QueryContext(ctx, `SELECT c.id, c.material_id, m.original_filename, c.chunk_index,
		c.start_offset, c.end_offset, c.chunk_text,
		MATCH(c.chunk_text) AGAINST (? IN NATURAL LANGUAGE MODE) AS relevance
		FROM knowledge_chunks c JOIN materials m ON m.id = c.material_id AND m.class_id = c.class_id
		WHERE c.class_id = ? AND c.index_status IN ('ready', 'failed')
		AND MATCH(c.chunk_text) AGAINST (? IN NATURAL LANGUAGE MODE) > 0
		ORDER BY relevance DESC, c.id ASC LIMIT ?`, term, classID, term, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := make([]Hit, 0)
	for rows.Next() {
		var hit Hit
		if err := rows.Scan(&hit.ChunkID, &hit.MaterialID, &hit.Title, &hit.ChunkIndex, &hit.Start, &hit.End, &hit.ChunkText, &hit.Score); err != nil {
			return nil, err
		}
		hit.Snippet = excerpt(hit.ChunkText)
		hits = append(hits, hit)
	}
	return hits, rows.Err()
}

func (s *Service) vector(ctx context.Context, classID int64, term string, limit int) ([]Hit, error) {
	vector, err := s.embedder.Embed(ctx, term)
	if err != nil {
		return nil, err
	}
	points, err := s.vectors.Query(ctx, classID, vector, limit*2)
	if err != nil {
		return nil, err
	}
	hits := make([]Hit, 0, limit)
	for _, point := range points {
		if point.Score < 0.35 {
			continue
		}
		var hit Hit
		err := s.database.QueryRowContext(ctx, `SELECT c.id, c.material_id, m.original_filename, c.chunk_index,
			c.start_offset, c.end_offset, c.chunk_text
			FROM knowledge_chunks c JOIN materials m ON m.id = c.material_id AND m.class_id = c.class_id
			WHERE c.id = ? AND c.class_id = ? AND c.index_status = 'ready'`, point.ID, classID).
			Scan(&hit.ChunkID, &hit.MaterialID, &hit.Title, &hit.ChunkIndex, &hit.Start, &hit.End, &hit.ChunkText)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		hit.Score, hit.Snippet = point.Score, excerpt(hit.ChunkText)
		hits = append(hits, hit)
		if len(hits) == limit {
			break
		}
	}
	return hits, nil
}

func fuse(keywords, vectors []Hit, limit int) []Hit {
	type ranked struct {
		hit Hit
		rrf float64
	}
	merged := make(map[int64]*ranked)
	for _, list := range [][]Hit{keywords, vectors} {
		for rank, hit := range list {
			entry := merged[hit.ChunkID]
			if entry == nil {
				entry = &ranked{hit: hit}
				merged[hit.ChunkID] = entry
			}
			entry.rrf += 1 / float64(60+rank+1)
		}
	}
	all := make([]ranked, 0, len(merged))
	for _, entry := range merged {
		entry.hit.Score = entry.rrf
		all = append(all, *entry)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].rrf == all[j].rrf {
			return all[i].hit.ChunkID < all[j].hit.ChunkID
		}
		return all[i].rrf > all[j].rrf
	})
	hits := make([]Hit, 0, min(limit, len(all)))
	for i := 0; i < len(all) && i < limit; i++ {
		hits = append(hits, all[i].hit)
	}
	return hits
}

func excerpt(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) > 180 {
		return string(runes[:180]) + "…"
	}
	return string(runes)
}
