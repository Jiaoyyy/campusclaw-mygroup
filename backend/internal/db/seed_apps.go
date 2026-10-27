package db

import (
	"context"
	"database/sql"
	"errors"

	"campusclaw/backend/internal/aicatalog"
)

// Existing teacher edits and bindings are never overwritten on restart.
func ensureApplications(ctx context.Context, tx *sql.Tx, classID int64) error {
	for _, app := range aicatalog.Applications() {
		var skillID int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM tutor_skills WHERE class_id=? AND name=?`, classID, app.SkillName).Scan(&skillID)
		if errors.Is(err, sql.ErrNoRows) {
			result, err := tx.ExecContext(ctx, `INSERT INTO tutor_skills(class_id,name,trigger_text,subject_text,description,source_text,enabled) VALUES(?,?,?,'',?,?,TRUE)`, classID, app.SkillName, app.SkillName, app.SkillSource, app.SkillSource)
			if err != nil {
				return err
			}
			skillID, err = result.LastInsertId()
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		var agentID int64
		err = tx.QueryRowContext(ctx, `SELECT id FROM tutor_agents WHERE class_id=? AND name=?`, classID, app.Name).Scan(&agentID)
		if err == nil {
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO tutor_agents(class_id,name,prompt,enabled) VALUES(?,?,?,TRUE)`, classID, app.Name, app.Prompt)
		if err != nil {
			return err
		}
		agentID, err = result.LastInsertId()
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO tutor_agent_skills(agent_id,skill_id,class_id) VALUES(?,?,?)`, agentID, skillID, classID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO tutor_agent_materials(agent_id,material_id,class_id) SELECT ?,id,class_id FROM materials WHERE class_id=? AND original_filename IN ('A班课程提纲.txt','B班实验须知.md')`, agentID, classID); err != nil {
			return err
		}
	}
	return nil
}
