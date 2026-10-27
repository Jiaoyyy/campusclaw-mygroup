CREATE TABLE IF NOT EXISTS tutor_agent_mcp_tools (
 agent_id BIGINT UNSIGNED NOT NULL,
 class_id BIGINT UNSIGNED NOT NULL,
 tool_name VARCHAR(128) NOT NULL,
 PRIMARY KEY (agent_id, tool_name),
 CONSTRAINT fk_tutor_agent_mcp_tool_agent FOREIGN KEY (agent_id, class_id) REFERENCES tutor_agents(id, class_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tutor_message_mcp_tools (
 message_id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
 tool_name VARCHAR(128) NOT NULL,
 CONSTRAINT fk_tutor_message_mcp_tool_message FOREIGN KEY (message_id) REFERENCES tutor_messages(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
