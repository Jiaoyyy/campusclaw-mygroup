package tutor

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"campusclaw/backend/internal/aicatalog"
	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/config"
	"campusclaw/backend/internal/retrieval"
)

type Service struct {
	db        *sql.DB
	auth      *auth.Service
	retrieval *retrieval.Service
	cfg       config.Config
}

func New(db *sql.DB, authentication *auth.Service, search *retrieval.Service, cfg config.Config) *Service {
	return &Service{db: db, auth: authentication, retrieval: search, cfg: cfg}
}

func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/prompt-templates", s.listPromptTemplates)
	mux.HandleFunc("GET /api/skills", s.listSkills)
	mux.HandleFunc("POST /api/skills", s.saveSkill)
	mux.HandleFunc("GET /api/skills/{id}", s.getSkill)
	mux.HandleFunc("PUT /api/skills/{id}", s.saveSkill)
	mux.HandleFunc("DELETE /api/skills/{id}", s.deleteSkill)
	mux.HandleFunc("GET /api/agents", s.listAgents)
	mux.HandleFunc("POST /api/agents", s.saveAgent)
	mux.HandleFunc("GET /api/agents/{id}", s.getAgent)
	mux.HandleFunc("PUT /api/agents/{id}", s.saveAgent)
	mux.HandleFunc("GET /api/conversations", s.listConversations)
	mux.HandleFunc("POST /api/conversations", s.createConversation)
	mux.HandleFunc("GET /api/conversations/{id}", s.getConversation)
	mux.HandleFunc("POST /api/conversations/{id}/messages", s.sendMessage)
	mux.HandleFunc("GET /api/mcp/tools", s.listMCPTools)
}

func (s *Service) listPromptTemplates(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.identity(w, r, false, false); !ok {
		return
	}
	auth.WriteJSON(w, 200, map[string]any{"templates": aicatalog.Templates()})
}

func (s *Service) listMCPTools(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.identity(w, r, true, false); !ok {
		return
	}
	if !mcpConfigured(s.cfg) {
		auth.WriteJSON(w, 200, map[string]any{"status": "disabled", "tools": []MCPTool{}})
		return
	}
	tools, err := discoverMCPTools(r.Context(), s.cfg)
	if err != nil {
		auth.WriteJSON(w, 200, map[string]any{"status": "unavailable", "tools": []MCPTool{}})
		return
	}
	auth.WriteJSON(w, 200, map[string]any{"status": "connected", "tools": tools})
}

func (s *Service) identity(w http.ResponseWriter, r *http.Request, teacher, write bool) (auth.Identity, bool) {
	id, token, status := s.auth.Current(r)
	if status != 0 {
		fail(w, status, "authentication required")
		return id, false
	}
	if teacher && id.Role != "teacher" {
		fail(w, http.StatusForbidden, "teacher role required")
		return id, false
	}
	if write && !s.auth.ValidCSRF(r, token) {
		fail(w, http.StatusForbidden, "invalid csrf token")
		return id, false
	}
	return id, true
}

func fail(w http.ResponseWriter, status int, message string) { auth.WriteError(w, status, message) }
func unavailable(w http.ResponseWriter) {
	fail(w, http.StatusServiceUnavailable, "service unavailable")
}
func notFound(w http.ResponseWriter) { fail(w, http.StatusNotFound, "not found") }
func number(r *http.Request) (int64, bool) {
	n, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return n, err == nil && n > 0
}
func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(dst) != nil || dec.Decode(new(any)) != io.EOF {
		fail(w, 400, "invalid request")
		return false
	}
	return true
}
func length(s string) int { return utf8.RuneCountInString(s) }
func dbConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Duplicate entry")
}
func dbErr(w http.ResponseWriter, err error) {
	if dbConflict(err) {
		fail(w, 400, "name already exists in this class")
	} else {
		unavailable(w)
	}
}

type Skill struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	TriggerText string `json:"trigger_text"`
	SubjectText string `json:"subject_text"`
	Description string `json:"description"`
	SourceText  string `json:"source_text"`
	Enabled     bool   `json:"enabled"`
}

func parseSkill(text string) (Skill, error) {
	var out Skill
	original := text
	if length(text) > 8000 {
		return out, errors.New("skill file exceeds 8000 characters")
	}
	if strings.TrimSpace(text) == "" {
		return out, errors.New("skill file is empty")
	}
	// Keep the original Markdown intact. Legacy field-based files remain readable.
	if !strings.HasPrefix(strings.TrimSpace(text), "名称：") && !strings.HasPrefix(strings.TrimSpace(text), "名称:") {
		lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
		if len(lines) > 2 && strings.TrimSpace(lines[0]) == "---" {
			for _, line := range lines[1:] {
				if strings.TrimSpace(line) == "---" {
					break
				}
				if value, ok := strings.CutPrefix(strings.TrimSpace(line), "name:"); ok {
					out.Name = strings.Trim(strings.TrimSpace(value), "\"'")
				}
			}
		}
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if out.Name == "" && strings.HasPrefix(line, "# ") {
				out.Name = strings.TrimSpace(strings.TrimPrefix(line, "# "))
				break
			}
		}
		if out.Name == "" || length(out.Name) > 32 {
			return out, errors.New("skill Markdown needs a # title or frontmatter name of at most 32 characters")
		}
		out.TriggerText = out.Name
		out.Description = text
		out.SourceText = text
		out.Enabled = true
		return out, nil
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	head, body, ok := strings.Cut(text, "\n\n")
	if !ok || strings.TrimSpace(body) == "" {
		return out, errors.New("skill description is required after a blank line")
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(head, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, found := strings.Cut(line, "：")
		if !found {
			k, v, found = strings.Cut(line, ":")
		}
		if !found {
			return out, errors.New("invalid skill metadata")
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if seen[k] {
			return out, errors.New("duplicate skill field: " + k)
		}
		seen[k] = true
		switch k {
		case "名称":
			out.Name = v
		case "适用条件":
			out.TriggerText = v
		case "资料":
			out.SubjectText = v
		case "工具":
			if v != "knowledge_search" {
				return out, errors.New("only knowledge_search is supported")
			}
		case "启用":
			if v != "是" && v != "否" {
				return out, errors.New("启用 must be 是 or 否")
			}
			out.Enabled = v == "是"
		default:
			return out, errors.New("unknown skill field: " + k)
		}
	}
	if out.Name == "" || out.TriggerText == "" || !seen["启用"] {
		return out, errors.New("名称、适用条件和启用 are required")
	}
	out.Description, out.SourceText = strings.TrimSpace(body), original
	if length(out.Name) > 32 || length(out.TriggerText) > 120 || length(out.SubjectText) > 32 || length(out.Description) > 2000 {
		return out, errors.New("skill field exceeds length limit")
	}
	return out, nil
}

func (s *Service) listSkills(w http.ResponseWriter, r *http.Request) {
	id, ok := s.identity(w, r, false, false)
	if !ok {
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT id,name,trigger_text,subject_text,description,source_text,enabled FROM tutor_skills WHERE class_id=? ORDER BY id`, id.ClassID)
	if err != nil {
		unavailable(w)
		return
	}
	defer rows.Close()
	items := []Skill{}
	for rows.Next() {
		var x Skill
		if rows.Scan(&x.ID, &x.Name, &x.TriggerText, &x.SubjectText, &x.Description, &x.SourceText, &x.Enabled) != nil {
			unavailable(w)
			return
		}
		items = append(items, x)
	}
	if rows.Err() != nil {
		unavailable(w)
		return
	}
	auth.WriteJSON(w, 200, map[string]any{"skills": items})
}
func (s *Service) getSkill(w http.ResponseWriter, r *http.Request) {
	id, ok := s.identity(w, r, false, false)
	if !ok {
		return
	}
	n, valid := number(r)
	if !valid {
		notFound(w)
		return
	}
	var x Skill
	err := s.db.QueryRowContext(r.Context(), `SELECT id,name,trigger_text,subject_text,description,source_text,enabled FROM tutor_skills WHERE id=? AND class_id=?`, n, id.ClassID).Scan(&x.ID, &x.Name, &x.TriggerText, &x.SubjectText, &x.Description, &x.SourceText, &x.Enabled)
	if errors.Is(err, sql.ErrNoRows) {
		notFound(w)
		return
	}
	if err != nil {
		unavailable(w)
		return
	}
	auth.WriteJSON(w, 200, x)
}
func (s *Service) saveSkill(w http.ResponseWriter, r *http.Request) {
	id, ok := s.identity(w, r, true, true)
	if !ok {
		return
	}
	var input struct {
		SourceText string `json:"source_text"`
		Enabled    *bool  `json:"enabled"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	x, err := parseSkill(input.SourceText)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	if input.Enabled != nil {
		x.Enabled = *input.Enabled
	}
	if r.Method == http.MethodPost {
		result, err := s.db.ExecContext(r.Context(), `INSERT INTO tutor_skills(class_id,name,trigger_text,subject_text,description,source_text,enabled) VALUES(?,?,?,?,?,?,?)`, id.ClassID, x.Name, x.TriggerText, x.SubjectText, x.Description, x.SourceText, x.Enabled)
		if err != nil {
			dbErr(w, err)
			return
		}
		x.ID, _ = result.LastInsertId()
		auth.WriteJSON(w, 201, x)
		return
	}
	n, valid := number(r)
	if !valid {
		notFound(w)
		return
	}
	x.ID = n
	result, err := s.db.ExecContext(r.Context(), `UPDATE tutor_skills SET name=?,trigger_text=?,subject_text=?,description=?,source_text=?,enabled=? WHERE id=? AND class_id=?`, x.Name, x.TriggerText, x.SubjectText, x.Description, x.SourceText, x.Enabled, n, id.ClassID)
	if err != nil {
		dbErr(w, err)
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		notFound(w)
		return
	}
	auth.WriteJSON(w, 200, x)
}
func (s *Service) deleteSkill(w http.ResponseWriter, r *http.Request) {
	id, ok := s.identity(w, r, true, true)
	if !ok {
		return
	}
	n, valid := number(r)
	if !valid {
		notFound(w)
		return
	}
	result, err := s.db.ExecContext(r.Context(), `DELETE FROM tutor_skills WHERE id=? AND class_id=?`, n, id.ClassID)
	if err != nil {
		unavailable(w)
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		notFound(w)
		return
	}
	auth.WriteJSON(w, 200, map[string]string{"status": "ok"})
}
