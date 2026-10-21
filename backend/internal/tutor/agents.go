package tutor

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"campusclaw/backend/internal/auth"
)

type Agent struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Prompt      string  `json:"prompt"`
	Enabled     bool    `json:"enabled"`
	SkillIDs    []int64 `json:"skill_ids"`
	MaterialIDs []int64 `json:"material_ids"`
}

func (s *Service) agent(r *http.Request, classID, agentID int64, student bool) (Agent, error) {
	var a Agent
	a.SkillIDs, a.MaterialIDs = []int64{}, []int64{}
	query := `SELECT id,name,prompt,enabled FROM tutor_agents WHERE id=? AND class_id=?`
	if student {
		query += ` AND enabled=TRUE`
	}
	err := s.db.QueryRowContext(r.Context(), query, agentID, classID).Scan(&a.ID, &a.Name, &a.Prompt, &a.Enabled)
	if err != nil {
		return a, err
	}
	for _, b := range []struct {
		table  string
		target *[]int64
	}{{"tutor_agent_skills", &a.SkillIDs}, {"tutor_agent_materials", &a.MaterialIDs}} {
		col := "skill_id"
		if b.table == "tutor_agent_materials" {
			col = "material_id"
		}
		rows, err := s.db.QueryContext(r.Context(), `SELECT `+col+` FROM `+b.table+` WHERE agent_id=? AND class_id=? ORDER BY `+col, agentID, classID)
		if err != nil {
			return a, err
		}
		for rows.Next() {
			var n int64
			if err = rows.Scan(&n); err != nil {
				break
			}
			*b.target = append(*b.target, n)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return a, err
		}
	}
	return a, nil
}

func (s *Service) getAgent(w http.ResponseWriter, r *http.Request) {
	id, ok := s.identity(w, r, false, false)
	if !ok {
		return
	}
	n, valid := number(r)
	if !valid {
		notFound(w)
		return
	}
	a, err := s.agent(r, id.ClassID, n, id.Role == "student")
	if errors.Is(err, sql.ErrNoRows) {
		notFound(w)
		return
	}
	if err != nil {
		unavailable(w)
		return
	}
	auth.WriteJSON(w, 200, a)
}
func (s *Service) listAgents(w http.ResponseWriter, r *http.Request) {
	id, ok := s.identity(w, r, false, false)
	if !ok {
		return
	}
	q := `SELECT id FROM tutor_agents WHERE class_id=?`
	if id.Role == "student" {
		q += ` AND enabled=TRUE`
	}
	q += ` ORDER BY id`
	rows, err := s.db.QueryContext(r.Context(), q, id.ClassID)
	if err != nil {
		unavailable(w)
		return
	}
	ids := []int64{}
	for rows.Next() {
		var n int64
		if rows.Scan(&n) != nil {
			rows.Close()
			unavailable(w)
			return
		}
		ids = append(ids, n)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		unavailable(w)
		return
	}
	agents := []Agent{}
	for _, n := range ids {
		a, err := s.agent(r, id.ClassID, n, id.Role == "student")
		if err != nil {
			unavailable(w)
			return
		}
		agents = append(agents, a)
	}
	auth.WriteJSON(w, 200, map[string]any{"agents": agents})
}
func (s *Service) saveAgent(w http.ResponseWriter, r *http.Request) {
	id, ok := s.identity(w, r, true, true)
	if !ok {
		return
	}
	var a Agent
	if !readJSON(w, r, &a) {
		return
	}
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" || length(a.Name) > 64 || length(a.Prompt) > 2000 {
		fail(w, 400, "invalid agent name or prompt")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		unavailable(w)
		return
	}
	defer tx.Rollback()
	if r.Method == http.MethodPost {
		result, e := tx.ExecContext(r.Context(), `INSERT INTO tutor_agents(class_id,name,prompt,enabled) VALUES(?,?,?,?)`, id.ClassID, a.Name, a.Prompt, a.Enabled)
		err = e
		if err == nil {
			a.ID, err = result.LastInsertId()
		}
	} else {
		n, valid := number(r)
		if !valid {
			notFound(w)
			return
		}
		a.ID = n
		var exists int
		err = tx.QueryRowContext(r.Context(), `SELECT 1 FROM tutor_agents WHERE id=? AND class_id=?`, n, id.ClassID).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			notFound(w)
			return
		}
		if err != nil {
			unavailable(w)
			return
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE tutor_agents SET name=?,prompt=?,enabled=? WHERE id=? AND class_id=?`, a.Name, a.Prompt, a.Enabled, n, id.ClassID)
	}
	if err != nil {
		dbErr(w, err)
		return
	}
	for _, binding := range []struct {
		ids                []int64
		table, col, source string
	}{{a.SkillIDs, "tutor_agent_skills", "skill_id", "tutor_skills"}, {a.MaterialIDs, "tutor_agent_materials", "material_id", "materials"}} {
		if _, err = tx.ExecContext(r.Context(), `DELETE FROM `+binding.table+` WHERE agent_id=? AND class_id=?`, a.ID, id.ClassID); err != nil {
			unavailable(w)
			return
		}
		seen := map[int64]bool{}
		for _, n := range binding.ids {
			if n <= 0 || seen[n] {
				fail(w, 400, "duplicate or invalid binding")
				return
			}
			seen[n] = true
			var exists int
			err = tx.QueryRowContext(r.Context(), `SELECT 1 FROM `+binding.source+` WHERE id=? AND class_id=?`, n, id.ClassID).Scan(&exists)
			if errors.Is(err, sql.ErrNoRows) {
				notFound(w)
				return
			}
			if err != nil {
				unavailable(w)
				return
			}
			if _, err = tx.ExecContext(r.Context(), `INSERT INTO `+binding.table+`(agent_id,`+binding.col+`,class_id) VALUES(?,?,?)`, a.ID, n, id.ClassID); err != nil {
				unavailable(w)
				return
			}
		}
	}
	if err = tx.Commit(); err != nil {
		unavailable(w)
		return
	}
	if a.SkillIDs == nil {
		a.SkillIDs = []int64{}
	}
	if a.MaterialIDs == nil {
		a.MaterialIDs = []int64{}
	}
	status := 200
	if r.Method == http.MethodPost {
		status = 201
	}
	auth.WriteJSON(w, status, a)
}
