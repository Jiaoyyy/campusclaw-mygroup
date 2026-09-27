CREATE TABLE IF NOT EXISTS classes (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(100) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_classes_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    username VARCHAR(100) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL,
    class_id BIGINT UNSIGNED NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_users_username (username),
    UNIQUE KEY uq_users_id_class (id, class_id),
    KEY idx_users_class (class_id),
    CONSTRAINT ck_users_role CHECK (role IN ('teacher', 'student')),
    CONSTRAINT fk_users_class FOREIGN KEY (class_id) REFERENCES classes (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS sessions (
    token_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    expires_at DATETIME(6) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (token_hash),
    KEY idx_sessions_user (user_id),
    KEY idx_sessions_expires (expires_at),
    CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS materials (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    class_id BIGINT UNSIGNED NOT NULL,
    uploader_id BIGINT UNSIGNED NULL,
    original_filename VARCHAR(255) NOT NULL,
    storage_key CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    mime VARCHAR(100) NOT NULL,
    size_bytes BIGINT UNSIGNED NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_materials_storage_key (storage_key),
    UNIQUE KEY uq_materials_id_class (id, class_id),
    KEY idx_materials_class_created (class_id, created_at, id),
    KEY idx_materials_uploader_class (uploader_id, class_id),
    CONSTRAINT fk_materials_class FOREIGN KEY (class_id) REFERENCES classes (id),
    CONSTRAINT fk_materials_uploader_class FOREIGN KEY (uploader_id, class_id)
        REFERENCES users (id, class_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS knowledge_entries (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    material_id BIGINT UNSIGNED NOT NULL,
    class_id BIGINT UNSIGNED NOT NULL,
    body LONGTEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_knowledge_material (material_id),
    KEY idx_knowledge_class (class_id),
    KEY idx_knowledge_material_class (material_id, class_id),
    CONSTRAINT fk_knowledge_class FOREIGN KEY (class_id) REFERENCES classes (id),
    CONSTRAINT fk_knowledge_material_class FOREIGN KEY (material_id, class_id)
        REFERENCES materials (id, class_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
