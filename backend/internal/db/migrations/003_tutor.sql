CREATE TABLE IF NOT EXISTS tutor_skills (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 class_id BIGINT UNSIGNED NOT NULL,
 name VARCHAR(32) NOT NULL,
 trigger_text VARCHAR(120) NOT NULL,
 subject_text VARCHAR(32) NOT NULL DEFAULT '',
 description TEXT NOT NULL,
 source_text TEXT NOT NULL,
 enabled BOOLEAN NOT NULL DEFAULT TRUE,
 created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE KEY uq_tutor_skills_class_name (class_id, name),
 UNIQUE KEY uq_tutor_skills_id_class (id, class_id),
 CONSTRAINT fk_tutor_skills_class FOREIGN KEY (class_id) REFERENCES classes(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tutor_agents (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 class_id BIGINT UNSIGNED NOT NULL,
 name VARCHAR(64) NOT NULL,
 prompt TEXT NOT NULL,
 enabled BOOLEAN NOT NULL DEFAULT TRUE,
 created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE KEY uq_tutor_agents_class_name (class_id, name),
 UNIQUE KEY uq_tutor_agents_id_class (id, class_id),
 CONSTRAINT fk_tutor_agents_class FOREIGN KEY (class_id) REFERENCES classes(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tutor_agent_skills (
 agent_id BIGINT UNSIGNED NOT NULL,
 skill_id BIGINT UNSIGNED NOT NULL,
 class_id BIGINT UNSIGNED NOT NULL,
 PRIMARY KEY (agent_id, skill_id),
 CONSTRAINT fk_tutor_agent_skill_agent FOREIGN KEY (agent_id, class_id) REFERENCES tutor_agents(id, class_id) ON DELETE CASCADE,
 CONSTRAINT fk_tutor_agent_skill_skill FOREIGN KEY (skill_id, class_id) REFERENCES tutor_skills(id, class_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tutor_agent_materials (
 agent_id BIGINT UNSIGNED NOT NULL,
 material_id BIGINT UNSIGNED NOT NULL,
 class_id BIGINT UNSIGNED NOT NULL,
 PRIMARY KEY (agent_id, material_id),
 CONSTRAINT fk_tutor_agent_material_agent FOREIGN KEY (agent_id, class_id) REFERENCES tutor_agents(id, class_id) ON DELETE CASCADE,
 CONSTRAINT fk_tutor_agent_material_material FOREIGN KEY (material_id, class_id) REFERENCES materials(id, class_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tutor_conversations (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 class_id BIGINT UNSIGNED NOT NULL,
 user_id BIGINT UNSIGNED NOT NULL,
 agent_id BIGINT UNSIGNED NOT NULL,
 title VARCHAR(100) NOT NULL DEFAULT '新的提问',
 created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 KEY idx_tutor_conversations_owner (user_id, class_id, updated_at),
 CONSTRAINT fk_tutor_conversation_user FOREIGN KEY (user_id, class_id) REFERENCES users(id, class_id),
 CONSTRAINT fk_tutor_conversation_agent FOREIGN KEY (agent_id, class_id) REFERENCES tutor_agents(id, class_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tutor_messages (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 conversation_id BIGINT UNSIGNED NOT NULL,
 role VARCHAR(12) NOT NULL,
 content TEXT NOT NULL,
 citations_json JSON NULL,
 skill_name VARCHAR(32) NULL,
 created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
 KEY idx_tutor_messages_conversation (conversation_id, id),
 CONSTRAINT ck_tutor_message_role CHECK (role IN ('user', 'assistant')),
 CONSTRAINT fk_tutor_message_conversation FOREIGN KEY (conversation_id) REFERENCES tutor_conversations(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
