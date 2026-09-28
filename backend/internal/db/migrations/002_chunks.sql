CREATE TABLE IF NOT EXISTS knowledge_chunks (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    knowledge_entry_id BIGINT UNSIGNED NOT NULL,
    material_id BIGINT UNSIGNED NOT NULL,
    class_id BIGINT UNSIGNED NOT NULL,
    chunk_index INT UNSIGNED NOT NULL,
    chunk_text TEXT NOT NULL,
    start_offset INT UNSIGNED NOT NULL,
    end_offset INT UNSIGNED NOT NULL,
    index_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    strategy VARCHAR(20) NOT NULL DEFAULT 'auto',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_chunks_entry_index (knowledge_entry_id, chunk_index),
    KEY idx_chunks_class_status (class_id, index_status),
    KEY idx_chunks_material_class (material_id, class_id),
    FULLTEXT KEY ft_chunks_text (chunk_text) WITH PARSER ngram,
    CONSTRAINT ck_chunks_status CHECK (index_status IN ('pending', 'ready', 'failed')),
    CONSTRAINT fk_chunks_entry FOREIGN KEY (knowledge_entry_id) REFERENCES knowledge_entries (id) ON DELETE CASCADE,
    CONSTRAINT fk_chunks_material_class FOREIGN KEY (material_id, class_id)
        REFERENCES materials (id, class_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
