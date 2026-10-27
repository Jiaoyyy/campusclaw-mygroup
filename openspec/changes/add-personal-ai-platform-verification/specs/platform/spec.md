# Personal AI Platform

## ADDED Requirements

### Requirement: Built-in applications and reusable prompts

每个初始化班级 SHALL 具备至少三个可用 AI 应用及相应内置 Markdown Skill。服务 SHALL 提供需登录的提示词模板目录；教师 SHALL 能选择、编辑并保存模板文本。初始化 MUST NOT 覆盖已有应用的提示词和绑定。

#### Scenario: Use a template
- **WHEN** 教师选择模板并保存智能体
- **THEN** 后续生成回答的 LLM 请求 SHALL 包含保存的完整角色提示词

### Requirement: Reproducible request verification

验证 SHALL 记录普通对话、会话历史、知识库与 Skill、MCP 工具选择与调用的真实请求和返回正文。证据 MUST NOT 包含凭据，并 SHALL 区分知识切片、选中技能原文、工具定义及工具结果。最终报告 SHALL 包含 Git 仓库地址、项目简介和五项结论。
