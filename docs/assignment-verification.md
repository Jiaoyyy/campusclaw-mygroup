# 个人项目验证记录

验证日期：2026-10-27；作者：焦阳。

仓库：https://github.com/Jiaoyyy/campusclaw-mygroup

## 新增功能

平台每班预置解题引导、知识复习、学习计划三个应用、三个提示词模板和三份完整 Markdown 技能。教师可选择模板后修改和保存，也可上传完整技能文件。初始化按名称补建，不覆盖已有应用与绑定；重启前后应用记录一致性核对通过。

教学助手的普通对话开关独立于嵌入模型，支持仅配置对话模型时的无资料对话或关键词检索。原 `/api/ask` 的向量问答前置条件不变。

## 实测结果

独立项目 `campusclaw-assignment-check` 使用人工资料、真实 `course-chat` 模型及公开术语 MCP 服务。五次 LLM 调用均返回 HTTP 200：普通对话一轮、同会话第二轮、知识与技能一轮、MCP 工具选择与最终生成两轮。

- 模板完整出现在最终生成请求的 `system.content`；同会话第二轮包含上一轮 user 和 assistant 消息。
- 8468 字的知识资料仅选择 4 个切片、3138 字正文；资料尾标记未进入请求。
- 选中技能的 110 字完整原文进入请求，尾标记存在，上传后原文读回相等。
- MCP 首次请求包含 `tools` 和 `tool_choice:auto`，不带知识正文和技能；模型返回 `tool_calls` 后，服务端实际调用 `course_glossary`，再发送不带 `tools` 的流式回答请求。最终回答解释了 RRF。
- 浏览器模板填入保存、完整 Markdown 导入、390px 宽度无横向溢出检查通过。
- 前端构建及 Go 全量单元测试通过；Go 的可选 MySQL 集成测试依旧需要 `TEST_MYSQL_DSN`，本次未把其跳过当作通过。数据库初始化与请求写入由独立环境的实际 API 运行验证。

## 验证边界

本次用于核对上下文传递的长资料采用关键词检索，独立环境关闭嵌入，以避免同步生成大量向量时超过代理超时。正常平台仍支持配置向量和混合检索。

本机虚拟 DNS 解析网关为不可达地址，本次按公开 DNS 结果用 `ASSIGNMENT_GATEWAY_IP` 对验证进程临时指定地址，仍保留原域名与 HTTPS 证书校验；未修改系统 DNS。IP 会变化，复现时应重新核对。

Skill 是提示词规则，不执行上传文件中的程序。只有选中的技能进入请求。单一 Markdown 技能默认使用，多个技能仍按词法路由，尚无语义自动路由。若给通用工具应用绑定强约束的专题技能，可能影响最终回答；本次已通过分开应用绑定验证这种边界。

## 交付物

- `output/pdf/campusclaw-personal-ai-platform-verification.pdf`：12 页报告，包含完整证据 JSON 附件。
- `output/pdf/campusclaw-personal-ai-platform-evidence.json`：独立的完整记录，不包含凭据。
- `scripts/verify_assignment.py`：真实请求记录和断言。
- `scripts/capture_assignment_ui.mjs`：实际浏览器操作验证。
- `scripts/build_assignment_report.py`：由证据生成 PDF。

最终 PDF 与独立证据 JSON 标明对应的代码提交，可据此在仓库核对实现。
