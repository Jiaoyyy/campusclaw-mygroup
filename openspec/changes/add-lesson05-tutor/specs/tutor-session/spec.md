# Tutor Session

## ADDED Requirements

### Requirement: Class-scoped agent and skill configuration

教师 SHALL 能维护本班智能体的名称、角色提示词、启用状态和技能/材料绑定，并维护至少一条从文本文件解析的「解题引导」技能。学生 MUST NOT 修改这些配置。跨班绑定 MUST 返回与不存在资源一致的 404。

#### Scenario: Skill routing
- **WHEN** 学生提问匹配多个已绑定、已启用技能
- **THEN** 服务端 SHALL 按适用条件词法得分选择至多一个；同分取编号较小者，未绑定或停用技能 MUST NOT 参与

### Requirement: Private conversations

新会话 SHALL 选定本班一个已启用智能体；会话及消息只能由创建者在本班读取。已有会话在智能体停用后仍可继续，但请求体不能改换智能体。模型上下文 SHALL 取最近六条有效消息，每条最多 500 字。

#### Scenario: Hidden conversation
- **WHEN** 用户读取他人、他班或不存在的会话
- **THEN** 三者 SHALL 返回完全一致的 404 响应

### Requirement: Bound retrieval and streaming answer

每轮回答 SHALL 先在所选智能体绑定的本班材料中检索，再通过 SSE 发送 `meta`、新增文本 `delta` 和持久化后的 `done`；流开始后失败 SHALL 发送 `error`。有证据时引用编号对应材料切片；无证据时仍按提示词回答，但引用为空、正文不含 `[n]`。

#### Scenario: Guiding skill
- **WHEN** 命中名称含「解题引导」的技能
- **THEN** 回答 SHALL 引导学生检查下一步，不直接提供最终答案

### Requirement: Inspectable API and verification

服务 SHALL 提供描述路径、参数、结果、错误状态和 Bearer 鉴权的 OpenAPI JSON 与浏览器 Swagger 页面。仓库 SHALL 提供按 URL 与方法组织的接口脚本、可一键生成的本地 HTML 报告，并至少核对登录、检索、提示词/技能、会话消息的接口返回值与数据库记录。

#### Scenario: API inspection and test report
- **WHEN** 开发者打开 `/swagger` 并运行 `apitest/run_all.py`
- **THEN** 页面 SHALL 能按 Bearer 文档试调主要接口，测试 SHALL 生成 Git 忽略的 HTML 报告并核对持久化数据
