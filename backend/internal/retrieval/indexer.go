package retrieval

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"campusclaw/backend/internal/chunking"
)

type Indexer struct {
	mu            sync.Mutex
	database      *sql.DB
	embedder      Embedder
	vectors       VectorStore
	vectorEnabled bool
}

func NewIndexer(database *sql.DB, embedder Embedder, vectors VectorStore, vectorEnabled bool) *Indexer {
	return &Indexer{database: database, embedder: embedder, vectors: vectors, vectorEnabled: vectorEnabled}
}

// IndexMaterial replaces one material's index. The original material and body
// are never removed if an external embedding or vector write fails.
func (s *Indexer) IndexMaterial(ctx context.Context, materialID, classID int64, options chunking.Options) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var entryID int64
	var body string
	err := s.database.QueryRowContext(ctx, `SELECT k.id, k.body FROM knowledge_entries k
		JOIN materials m ON m.id = k.material_id AND m.class_id = k.class_id
		WHERE k.material_id = ? AND k.class_id = ?`, materialID, classID).Scan(&entryID, &body)
	if err != nil {
		return fmt.Errorf("read material body: %w", err)
	}
	chunks, _, err := chunking.Split(body, options)
	if err != nil {
		return err
	}
	normalized, _ := options.Normalized()
	rows, err := s.database.QueryContext(ctx, "SELECT id FROM knowledge_chunks WHERE knowledge_entry_id = ? AND class_id = ?", entryID, classID)
	if err != nil {
		return fmt.Errorf("read old chunks: %w", err)
	}
	var oldIDs []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		oldIDs = append(oldIDs, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if err := s.vectors.Delete(ctx, oldIDs); err != nil && s.vectorEnabled {
		return fmt.Errorf("remove old vectors: %w", err)
	}
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM knowledge_chunks WHERE knowledge_entry_id = ? AND class_id = ?", entryID, classID); err != nil {
		return err
	}
	ids := make([]int64, 0, len(chunks))
	for _, chunk := range chunks {
		result, err := tx.ExecContext(ctx, `INSERT INTO knowledge_chunks
			(knowledge_entry_id, material_id, class_id, chunk_index, chunk_text, start_offset, end_offset, index_status, strategy)
			VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?)`, entryID, materialID, classID, chunk.Index, chunk.Text, chunk.Start, chunk.End, normalized.Strategy)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if !s.vectorEnabled {
		_, err := s.database.ExecContext(ctx, "UPDATE knowledge_chunks SET index_status = 'failed' WHERE knowledge_entry_id = ? AND class_id = ?", entryID, classID)
		return err
	}
	var firstError error
	for i, chunk := range chunks {
		vector, err := s.embedder.Embed(ctx, chunk.Text)
		if err == nil {
			err = s.vectors.Upsert(ctx, ids[i], vector, map[string]any{
				"class_id": classID, "material_id": materialID, "knowledge_entry_id": entryID,
				"chunk_id": ids[i], "chunk_index": chunk.Index,
			})
		}
		status := "ready"
		if err != nil {
			status = "failed"
			if firstError == nil {
				firstError = err
			}
		}
		if _, updateErr := s.database.ExecContext(ctx, "UPDATE knowledge_chunks SET index_status = ? WHERE id = ? AND class_id = ?", status, ids[i], classID); updateErr != nil {
			return updateErr
		}
	}
	return firstError
}

func (s *Indexer) Backfill(ctx context.Context) (int, error) {
	rows, err := s.database.QueryContext(ctx, `SELECT k.material_id, k.class_id FROM knowledge_entries k
		WHERE NOT EXISTS (SELECT 1 FROM knowledge_chunks c WHERE c.knowledge_entry_id = k.id)
		OR (? = 1 AND EXISTS (SELECT 1 FROM knowledge_chunks c WHERE c.knowledge_entry_id = k.id AND c.index_status IN ('failed', 'pending')))`, s.vectorEnabled)
	if err != nil {
		return 0, err
	}
	type item struct{ materialID, classID int64 }
	var items []item
	for rows.Next() {
		var one item
		if err := rows.Scan(&one.materialID, &one.classID); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, one)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	failed := 0
	for _, one := range items {
		var count int
		if err := s.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_chunks WHERE material_id = ? AND class_id = ?`, one.materialID, one.classID).Scan(&count); err != nil {
			return failed, err
		}
		var err error
		if count == 0 {
			err = s.IndexMaterial(ctx, one.materialID, one.classID, chunking.Options{})
		} else {
			err = s.retryChunks(ctx, one.materialID, one.classID)
		}
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return failed, err
			}
			failed++
		}
	}
	return failed, nil
}

// retryChunks keeps the original split and offsets. Re-splitting with the
// default strategy would silently discard a teacher's custom settings.
func (s *Indexer) retryChunks(ctx context.Context, materialID, classID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.database.QueryContext(ctx, `SELECT id, knowledge_entry_id, chunk_index, chunk_text
		FROM knowledge_chunks WHERE material_id = ? AND class_id = ?
		AND index_status IN ('failed', 'pending') ORDER BY chunk_index`, materialID, classID)
	if err != nil {
		return err
	}
	type pending struct {
		id, entryID int64
		index       int
		text        string
	}
	var chunks []pending
	for rows.Next() {
		var chunk pending
		if err := rows.Scan(&chunk.id, &chunk.entryID, &chunk.index, &chunk.text); err != nil {
			rows.Close()
			return err
		}
		chunks = append(chunks, chunk)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var firstError error
	for _, chunk := range chunks {
		vector, err := s.embedder.Embed(ctx, chunk.text)
		if err == nil {
			err = s.vectors.Upsert(ctx, chunk.id, vector, map[string]any{
				"class_id": classID, "material_id": materialID, "knowledge_entry_id": chunk.entryID,
				"chunk_id": chunk.id, "chunk_index": chunk.index,
			})
		}
		status := "ready"
		if err != nil {
			status = "failed"
			if firstError == nil {
				firstError = err
			}
		}
		if _, err := s.database.ExecContext(ctx, "UPDATE knowledge_chunks SET index_status = ? WHERE id = ? AND class_id = ?", status, chunk.id, classID); err != nil {
			return err
		}
	}
	return firstError
}
