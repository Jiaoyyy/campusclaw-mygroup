# Design

## Context

见 proposal.md 的动机。仓库目前只有 OpenSpec 文档，没有应用、依赖清单、测试或部署配置。本课实现登录、班级隔离和 TXT/Markdown 正文入库，采用课件示例的前后端分离技术栈与接口路径。

## Goals / Non-Goals

**Goals:** 浏览器只访问同源 Nginx 入口；Go 服务端统一判定身份、角色和班级；成功上传同时得到原文件、材料记录和本班知识库正文；Compose 能复现并验证成功及失败路径。

**Non-Goals:** 向量索引、语义或全文检索、RAG 问答、异步处理 worker、PDF/DOCX 解析、多班级账号、公开注册、材料编辑删除与生产多副本。前端材料列表的本班筛选不属于全文检索。

## Decisions

### 1. 请求链路与技术选型

前端使用 React 18、TypeScript 和 Vite，Go 标准库 `net/http` 提供 API，MySQL 8.0 保存业务数据，Nginx 托管前端构建产物并将 `/api` 和 `/health` 反向代理至 Go。浏览器只访问 Nginx 的同源入口，开发环境 Vite 的 `/api` 代理与 Compose 保持一致。Compose 服务为 web、api、db，只有 web 对宿主映射端口；数据库和上传卷不公开。备选是服务端模板单体，但本课选前后端分离以匹配课件请求链路。前端不得直连数据库或将上传目录当作静态资源。

### 2. 身份、会话与班级来源

密码使用 bcrypt 加盐慢哈希。登录成功签发新的随机不透明会话 ID，并作废旧会话；数据库会话表只保存会话摘要、用户 ID 与到期时间。Cookie 设置 HttpOnly、SameSite=Lax，本地 HTTP 不强制 Secure，生产 HTTPS 启用 Secure。登出删除服务端会话。每次业务请求根据会话从用户表读取 user_id、role、class_id，不采信请求头、请求体或前端状态中的角色和班级。Cookie 写操作校验 CSRF；登录失败按用户名与 IP 限流，账号不存在、密码错误及锁定期返回同形错误，并对不存在账号执行等价耗时的哈希比较。备选 JWT 不利于即时登出及服务端角色变更，本课不使用。

`SESSION_SECRET`、数据库凭据、种子账号口令等只从服务端环境变量读取，缺必填项时启动失败，不内置默认值，不记录明文口令、Cookie 或密钥。`.env.example` 只含占位符，`.gitignore` 和 `.dockerignore` 排除 `.env` 与上传目录。

### 3. 数据模型、幂等种子与隔离

- `classes`：id、name。
- `users`：id、唯一 username、password_hash、role（teacher/student）、非空 class_id 外键。
- `sessions`：token_hash、user_id、expires_at。
- `materials`：id、非空且有索引的 class_id、uploader_id、原文件名、随机 storage_key、mime、size、created_at。
- `knowledge_entries`：material_id 唯一外键、非空且有索引的 class_id、提取正文、created_at；约束正文与材料的班级一致。

幂等种子流程预置 A/B 两班、`teacher_a`、`student_a1`、`student_b1` 及标题可区分的两班材料与正文；口令由环境变量提供并在入库前哈希。重复执行不复制账号或材料，也不覆盖教师后续上传内容。没有公共注册接口。备选手工修改数据库无法稳定复现双班验收。

材料列表与计数的 SQL 必须带会话班级条件。按 ID 请求材料时先查行、再比较班级；不存在或跨班都返回同一 404，不返回标题、正文、文件路径等元数据。备选 403 会透露资源存在性。上传前先验证教师角色；携带客户端指定的 class_id 或 role 覆盖字段时拒绝。学生直接调用上传 API 返回 403，且不写入文件或数据库。前端隐藏上传入口只用于界面体验。

### 4. API 与页面

| 接口 | 行为 |
| --- | --- |
| POST /api/login | 账号密码登录，成功签发新会话；错误统一 401 |
| POST /api/logout | 删除当前服务端会话并清 Cookie |
| GET /api/me | 返回本人 ID、角色和班级 |
| GET /api/materials | 本班材料列表与本班范围内的简单筛选 |
| GET /api/materials/{id} | 本班材料详情及知识库正文，跨班 404 |
| GET /api/materials/{id}/file | 授权下载原文件，跨班 404 |
| POST /api/materials | 教师上传 TXT/Markdown，成功 201 |
| GET /health | 无需登录的进程存活检查 |

前端 SPA 包括登录页、材料列表、教师上传入口和材料详情。无会话的页面引导登录，受保护 API 返回 JSON 401 且无材料内容。Markdown 正文使用安全渲染，不执行材料中的 HTML 或脚本。列表数据来自数据库，不硬编码演示材料；简单筛选只在已授权本班数据范围内执行。

### 5. 上传事务与失败清理

上传顺序：验证会话、教师角色与 CSRF → 只允许 `.txt`/`.md` → 根据环境变量 `MAX_UPLOAD_BYTES` 限制流式读取，超限在读完整请求体前返回 413 → 校验非空 UTF-8 正文 → 以服务端随机文件名写入受保护上传目录 → 在同一 MySQL 事务写入本班 `materials` 与 `knowledge_entries` → 提交后返回 201 和材料 ID。原始文件名只用于展示，不参与构造存储路径。空文件、无效编码或无正文返回 400；不支持的扩展名返回 400。任何数据库或磁盘失败都回滚数据库并删除已写文件，不留下孤儿记录或文件；启动时清理超过宽限期的孤立临时文件，覆盖进程崩溃窗口。

备选异步 worker 与消息队列会增加处理中状态和重试机制。本课只解析 UTF-8 TXT/Markdown，同步写入可以在成功响应前验证两表与文件已完整保存。原文件仅经鉴权 API 下载，Nginx 不暴露 `/uploads` 静态目录。

### 6. Compose、存活语义与运行配置

Compose 启动 web、api、db，数据库与上传目录使用具名卷。api 用普通数据库账号连接，数据库管理员口令不下发给业务进程；api 在启动时等待 MySQL 真正可连接，不能只依赖 `depends_on`。本课按单实例运行。`GET /health` 只检查进程可响应，返回 200/status=ok，不探测数据库或文件卷；数据库故障时受保护业务 API 返回 503，不把已登录用户误判为 401。备选将数据库检查放进存活探针，会在数据库抖动时触发错误重启。README 说明 `.env.example` 配置、Compose 启动、种子账号、单实例边界、跨班 404、备份和不删卷回滚。

## Risks / Trade-offs

- [数据库事务无法与文件系统组成同一原子事务] → 成功响应前确认两者已写入，异常时补偿删除文件，并清理孤立临时文件。
- [仅靠页面隐藏按钮会遗漏越权] → 在 Go API 中统一认证授权，使用 A/B 两班及教师/学生账号执行直接 HTTP 负向测试。
- [不可信 Markdown 可注入脚本] → 安全渲染或转义正文，验证恶意内容不会执行。
- [进程内限流无法用于多副本] → 本课声明单实例，后续扩展需共享状态存储。

## Migration Plan

这是首个应用，没有既有业务数据需迁移。先建立 MySQL 表结构并运行幂等种子，再启动 API 与前端，按 README 执行两班验收。回滚时停止服务并保留具名卷；禁止通过删除卷来回滚。README 提供启动、健康检查、日志排查与备份步骤。
