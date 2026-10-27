package tutor

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/retrieval"
)

func (s *Service) useMCPForQuestion(ctx context.Context, boundNames []string, question string) (string, string, string) {
	if len(boundNames) == 0 || !mcpConfigured(s.cfg) {
		return "", "", ""
	}
	allowedForAgent := map[string]bool{}
	for _, name := range boundNames {
		allowedForAgent[name] = true
	}
	tools, err := discoverMCPTools(ctx, s.cfg)
	if err != nil {
		return "", "", "MCP 连接暂不可用，本轮未使用外部工具"
	}
	bound := make([]MCPTool, 0, len(tools))
	for _, tool := range tools {
		if allowedForAgent[tool.Name] {
			bound = append(bound, tool)
		}
	}
	if len(bound) == 0 {
		return "", "", "当前智能体绑定的 MCP 工具不可用"
	}
	name, args, err := chooseMCPTool(ctx, s.cfg, question, bound)
	if err != nil {
		return "", "", "外部工具选择失败，本轮仅使用本班材料"
	}
	if name == "" {
		return "", "", ""
	}
	result, err := callMCPTool(ctx, s.cfg, name, args)
	if err != nil {
		return "", "", "外部工具调用失败，本轮仅使用本班材料"
	}
	return name, result, ""
}

type Conversation struct {
	ID        int64     `json:"id"`
	AgentID   int64     `json:"agent_id"`
	Title     string    `json:"title"`
	UpdatedAt string    `json:"updated_at"`
	Messages  []Message `json:"messages,omitempty"`
}
type Message struct {
	ID          int64           `json:"id"`
	Role        string          `json:"role"`
	Content     string          `json:"content"`
	Citations   []retrieval.Hit `json:"citations"`
	SkillName   string          `json:"skill_name,omitempty"`
	MCPToolName string          `json:"mcp_tool_name,omitempty"`
}

func (s *Service) owned(r *http.Request, userID, classID, conversationID int64) (Conversation, error) {
	var c Conversation
	err := s.db.QueryRowContext(r.Context(), `SELECT id,agent_id,title,updated_at FROM tutor_conversations WHERE id=? AND user_id=? AND class_id=?`, conversationID, userID, classID).Scan(&c.ID, &c.AgentID, &c.Title, &c.UpdatedAt)
	return c, err
}
func (s *Service) listConversations(w http.ResponseWriter, r *http.Request) {
	id, ok := s.identity(w, r, false, false)
	if !ok {
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT id,agent_id,title,updated_at FROM tutor_conversations WHERE user_id=? AND class_id=? ORDER BY updated_at DESC,id DESC`, id.ID, id.ClassID)
	if err != nil {
		unavailable(w)
		return
	}
	defer rows.Close()
	items := []Conversation{}
	for rows.Next() {
		var c Conversation
		if rows.Scan(&c.ID, &c.AgentID, &c.Title, &c.UpdatedAt) != nil {
			unavailable(w)
			return
		}
		items = append(items, c)
	}
	if rows.Err() != nil {
		unavailable(w)
		return
	}
	auth.WriteJSON(w, 200, map[string]any{"conversations": items})
}
func (s *Service) createConversation(w http.ResponseWriter, r *http.Request) {
	id, ok := s.identity(w, r, false, true)
	if !ok {
		return
	}
	var input struct {
		AgentID int64 `json:"agent_id"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	var available int
	err := s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tutor_agents WHERE class_id=? AND enabled=TRUE`, id.ClassID).Scan(&available)
	if err != nil {
		unavailable(w)
		return
	}
	if available == 0 {
		fail(w, 400, "本班暂无可用的智能体")
		return
	}
	var exists int
	err = s.db.QueryRowContext(r.Context(), `SELECT 1 FROM tutor_agents WHERE id=? AND class_id=? AND enabled=TRUE`, input.AgentID, id.ClassID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		notFound(w)
		return
	}
	if err != nil {
		unavailable(w)
		return
	}
	result, err := s.db.ExecContext(r.Context(), `INSERT INTO tutor_conversations(class_id,user_id,agent_id,title) VALUES(?,?,?,'新的提问')`, id.ClassID, id.ID, input.AgentID)
	if err != nil {
		unavailable(w)
		return
	}
	n, _ := result.LastInsertId()
	c, err := s.owned(r, id.ID, id.ClassID, n)
	if err != nil {
		unavailable(w)
		return
	}
	auth.WriteJSON(w, 201, c)
}
func (s *Service) getConversation(w http.ResponseWriter, r *http.Request) {
	id, ok := s.identity(w, r, false, false)
	if !ok {
		return
	}
	n, valid := number(r)
	if !valid {
		notFound(w)
		return
	}
	c, err := s.owned(r, id.ID, id.ClassID, n)
	if errors.Is(err, sql.ErrNoRows) {
		notFound(w)
		return
	}
	if err != nil {
		unavailable(w)
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT m.id,m.role,m.content,m.citations_json,m.skill_name,t.tool_name FROM tutor_messages m LEFT JOIN tutor_message_mcp_tools t ON t.message_id=m.id WHERE m.conversation_id=? ORDER BY m.id`, n)
	if err != nil {
		unavailable(w)
		return
	}
	defer rows.Close()
	c.Messages = []Message{}
	for rows.Next() {
		var m Message
		var citationJSON []byte
		var skill, tool sql.NullString
		if rows.Scan(&m.ID, &m.Role, &m.Content, &citationJSON, &skill, &tool) != nil {
			unavailable(w)
			return
		}
		m.Citations = []retrieval.Hit{}
		if len(citationJSON) > 0 && json.Unmarshal(citationJSON, &m.Citations) != nil {
			unavailable(w)
			return
		}
		m.SkillName = skill.String
		m.MCPToolName = tool.String
		c.Messages = append(c.Messages, m)
	}
	if rows.Err() != nil {
		unavailable(w)
		return
	}
	auth.WriteJSON(w, 200, c)
}

func route(question string, skills []Skill) *Skill {
	sort.Slice(skills, func(i, j int) bool { return skills[i].ID < skills[j].ID })
	if len(skills) == 1 && !strings.HasPrefix(strings.TrimSpace(skills[0].SourceText), "名称：") && !strings.HasPrefix(strings.TrimSpace(skills[0].SourceText), "名称:") {
		return &skills[0]
	}
	text := strings.ToLower(question)
	best := 0
	var selected *Skill
	for i := range skills {
		score := 0
		for _, word := range strings.FieldsFunc(skills[i].TriggerText, func(r rune) bool { return strings.ContainsRune(",，、;；|", r) }) {
			word = strings.ToLower(strings.TrimSpace(word))
			if word != "" && strings.Contains(text, word) {
				score++
			}
		}
		if score > best {
			best = score
			selected = &skills[i]
		}
	}
	return selected
}
func (s *Service) boundSkills(r *http.Request, classID, agentID int64) ([]Skill, error) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT k.id,k.name,k.trigger_text,k.subject_text,k.description,k.source_text,k.enabled FROM tutor_skills k JOIN tutor_agent_skills b ON b.skill_id=k.id AND b.class_id=k.class_id WHERE b.agent_id=? AND b.class_id=? AND k.enabled=TRUE ORDER BY k.id`, agentID, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Skill{}
	for rows.Next() {
		var x Skill
		if err = rows.Scan(&x.ID, &x.Name, &x.TriggerText, &x.SubjectText, &x.Description, &x.SourceText, &x.Enabled); err != nil {
			return nil, err
		}
		items = append(items, x)
	}
	return items, rows.Err()
}
func (s *Service) history(r *http.Request, conversationID int64) ([]ChatMessage, error) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT role,content FROM (SELECT id,role,content FROM tutor_messages WHERE conversation_id=? ORDER BY id DESC LIMIT 6) recent ORDER BY id ASC`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ChatMessage{}
	for rows.Next() {
		var role, content string
		if err = rows.Scan(&role, &content); err != nil {
			return nil, err
		}
		if role != "user" && role != "assistant" {
			continue
		}
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}
		runes := []rune(content)
		if len(runes) > 500 {
			content = string(runes[:500])
		}
		out = append(out, ChatMessage{Role: role, Content: content})
	}
	return out, rows.Err()
}
func (s *Service) sendMessage(w http.ResponseWriter, r *http.Request) {
	id, ok := s.identity(w, r, false, true)
	if !ok {
		return
	}
	n, valid := number(r)
	if !valid {
		notFound(w)
		return
	}
	c, err := s.owned(r, id.ID, id.ClassID, n)
	if errors.Is(err, sql.ErrNoRows) {
		notFound(w)
		return
	}
	if err != nil {
		unavailable(w)
		return
	}
	var input struct {
		Content string `json:"content"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	question := strings.TrimSpace(input.Content)
	if length(question) < 2 || length(question) > 500 {
		fail(w, 400, "question must be 2-500 characters")
		return
	}
	a, err := s.agent(r, id.ClassID, c.AgentID, false)
	if errors.Is(err, sql.ErrNoRows) {
		notFound(w)
		return
	}
	if err != nil {
		unavailable(w)
		return
	}
	skills, err := s.boundSkills(r, id.ClassID, a.ID)
	if err != nil {
		unavailable(w)
		return
	}
	skill := route(question, skills)
	if !s.cfg.ChatEnabled() {
		unavailable(w)
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		unavailable(w)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO tutor_messages(conversation_id,role,content) VALUES(?,'user',?)`, n, question); err != nil {
		unavailable(w)
		return
	}
	title := c.Title
	if title == "新的提问" {
		runes := []rune(question)
		if len(runes) > 24 {
			runes = runes[:24]
		}
		title = string(runes)
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE tutor_conversations SET title=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`, title, n); err != nil {
		unavailable(w)
		return
	}
	if err = tx.Commit(); err != nil {
		unavailable(w)
		return
	}
	hits, err := s.retrieval.SearchBound(r.Context(), id.ClassID, question, a.MaterialIDs, 4)
	if err != nil {
		unavailable(w)
		return
	}
	citations := []retrieval.Hit{}
	for _, hit := range hits {
		if strings.TrimSpace(hit.ChunkText) == "" {
			continue
		}
		if skill != nil && skill.SubjectText != "" && !strings.Contains(hit.Title, skill.SubjectText) {
			continue
		}
		hit.CitationNumber = len(citations) + 1
		citations = append(citations, hit)
	}
	history, err := s.history(r, n)
	if err != nil {
		unavailable(w)
		return
	}
	toolName, toolResult, toolWarning := s.useMCPForQuestion(r.Context(), a.MCPToolNames, question)
	messages := assembleWithMCP(a, skill, question, history, citations, toolName, toolResult)
	flusher, ok := w.(http.Flusher)
	if !ok {
		unavailable(w)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	skillName := ""
	if skill != nil {
		skillName = skill.Name
	}
	writeEvent(w, flusher, "meta", map[string]any{"status": "ok", "skill": map[string]string{"name": skillName}, "citations": citations, "mcp_tool_name": toolName, "mcp_warning": toolWarning})
	var full strings.Builder
	pending := ""
	emit := func(chunk string) error {
		if len(citations) == 0 {
			var clean string
			clean, pending = withoutCitations(pending + chunk)
			chunk = clean
		}
		if chunk != "" {
			full.WriteString(chunk)
			writeEvent(w, flusher, "delta", map[string]string{"text": chunk})
		}
		return nil
	}
	err = StreamChat(r.Context(), s.cfg, messages, emit)
	if err != nil {
		writeEvent(w, flusher, "error", map[string]string{"error": "回答生成失败，请重试"})
		return
	}
	if pending != "" {
		clean, _ := withoutCitations(pending + " ")
		clean = strings.TrimSuffix(clean, " ")
		if clean != "" {
			full.WriteString(clean)
			writeEvent(w, flusher, "delta", map[string]string{"text": clean})
		}
	}
	answer := strings.TrimSpace(full.String())
	if answer == "" {
		writeEvent(w, flusher, "error", map[string]string{"error": "回答为空，请重试"})
		return
	}
	used := map[int]bool{}
	for _, m := range citationRE.FindAllStringSubmatch(answer, -1) {
		var number int
		fmt.Sscan(m[1], &number)
		used[number] = true
	}
	finalCitations := []retrieval.Hit{}
	for _, hit := range citations {
		if used[hit.CitationNumber] {
			finalCitations = append(finalCitations, hit)
		}
	}
	citationJSON, _ := json.Marshal(finalCitations)
	answerTx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeEvent(w, flusher, "error", map[string]string{"error": "保存回答失败，请重试"})
		return
	}
	defer answerTx.Rollback()
	result, err := answerTx.ExecContext(r.Context(), `INSERT INTO tutor_messages(conversation_id,role,content,citations_json,skill_name) VALUES(?,'assistant',?,?,?)`, n, answer, citationJSON, skillName)
	if err != nil {
		writeEvent(w, flusher, "error", map[string]string{"error": "保存回答失败，请重试"})
		return
	}
	messageID, _ := result.LastInsertId()
	if toolName != "" {
		if _, err = answerTx.ExecContext(r.Context(), `INSERT INTO tutor_message_mcp_tools(message_id,tool_name) VALUES(?,?)`, messageID, toolName); err != nil {
			writeEvent(w, flusher, "error", map[string]string{"error": "保存回答失败，请重试"})
			return
		}
	}
	if err = answerTx.Commit(); err != nil {
		writeEvent(w, flusher, "error", map[string]string{"error": "保存回答失败，请重试"})
		return
	}
	_, _ = s.db.ExecContext(r.Context(), `UPDATE tutor_conversations SET updated_at=CURRENT_TIMESTAMP WHERE id=?`, n)
	writeEvent(w, flusher, "done", map[string]any{"message_id": messageID, "citations": finalCitations})
}

var citationRE = regexp.MustCompile(`\[(\d+)\]`)

func withoutCitations(text string) (string, string) {
	hold := ""
	if i := strings.LastIndex(text, "["); i >= 0 && !strings.Contains(text[i:], "]") {
		hold = text[i:]
		text = text[:i]
	}
	return citationRE.ReplaceAllString(text, ""), hold
}
func writeEvent(w http.ResponseWriter, f http.Flusher, name string, data any) {
	body, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, body)
	f.Flush()
}
